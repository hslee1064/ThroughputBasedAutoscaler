package controller

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// type PrometheusResult struct {
// 	Status string `json:"status"`
// 	Data   struct {
// 		ResultType string `json:"resultType"`
// 		Result     []struct {
// 			Metric map[string]string `json:"metric"`
// 			Value  []interface{}     `json:"value"`
// 		} `json:"result"`
// 	} `json:"data"`
// }

// func GetPrometheusMetrics(deployments []string, query string) (PrometheusResult, error) {
// 	url := "http://prometheus-stack-kube-prom-prometheus.prometheus.svc.cluster.local:9090/api/v1/query?query="
// 	resp, err := http.Get(fmt.Sprintf("%s%s", url, query))
// 	if err != nil {
// 		return PrometheusResult{}, err
// 	}

// 	body, err := io.ReadAll(resp.Body)
// 	if err != nil {
// 		return PrometheusResult{}, err
// 	}

// 	var obj PrometheusResult
// 	json.Unmarshal(body, &obj)
// 	// println(obj.Data.Result[0].Metric["__name__"])
// 	// println(obj.Data.Result[0].Value[0].(float64))
// 	// println(obj.Data.Result[0].Value[1].(string))
// 	return obj, nil
// }

// func (s *AutoscalerEventSource) PollingMetric(ctx context.Context) error {
// 	for {
// 		var autoscalerList v1.AutoscalerList
// 		s.client.List(ctx, &autoscalerList)
// 		for _, item := range autoscalerList.Items {
// 			cpuQuery := "container_cpu_usage_seconds_total"
// 			metrics, err := GetPrometheusMetrics(item.Spec.Deployments, cpuQuery)
// 			if err != nil {
// 				return err
// 			}
// 			println(metrics.Data.Result)
// 		}
// 		println()
// 		time.Sleep(time.Second * 2)
// 	}
// }

func NewAutoscalerEventSource(client client.Client) (*AutoscalerEventSource, error) {
	return &AutoscalerEventSource{client: client}, nil
}

type AutoscalerEventSource struct {
	client client.Client
}

func (s *AutoscalerEventSource) PollingRedis(ctx context.Context) error {
	rdb := redis.NewClient(&redis.Options{
		// Addr:     "redis-master.redis.svc.cluster.local:6379",
		Addr:     "localhost:6379",
		Password: "1234",
	})

	AllGPU := s.GetAllGPU(ctx)

	for {
		keys := rdb.Keys(ctx, "*")
		streams := make(map[string]int)
		deployments := s.GetDeployments(ctx)

		redisStreamLog := "[Redis Stream Status]"
		redisThroughputLog := "[Redis Throughput Status]"
		redisThroughput := make(map[string]float64)
		for _, key := range keys.Val() {
			if strings.Contains(key, "stream") {
				length := int(rdb.XInfoGroups(ctx, key).Val()[0].Lag)
				println(key, length)
				redisStreamLog += fmt.Sprintf(" %s %d", key, length)
				if strings.Contains(key, "end") {
					continue
				}

				streams[key] = length
			} else if strings.Contains(key, "throughput") {
				val, _ := strconv.ParseFloat(rdb.Get(ctx, key).Val(), 64)
				redisThroughput[key] = val
				redisThroughputLog += fmt.Sprintf(" %s %f", key, val)
			}
		}
		println(redisStreamLog)
		println(redisThroughputLog)

		// Check Stream(queue) Length
		for key, queueLength := range streams {
			name := strings.Split(key, "-")[0]
			deployment := deployments[name]
			cpuReplicas := int(deployment[CPU].Status.AvailableReplicas)
			cpuThroughput := getIntervalThroughput(name, CPU, redisThroughput)
			gpuThroughput := getIntervalThroughput(name, GPU, redisThroughput)
			allocatedGPU := s.GetAllocatedGPU(ctx)
			allocatableGPU := AllGPU - allocatedGPU
			var gpuReplicas int
			if _, ok := deployment[GPU]; ok {
				gpuReplicas = int(deployment[GPU].Status.AvailableReplicas)
			}

			// Scaling depends on stream length
			modelThroughput := cpuThroughput*float64(cpuReplicas) + gpuThroughput*float64(gpuReplicas)
			if modelThroughput < float64(queueLength) {
				// ScaleUp
				scaleUpDevice := getScaleUpDevice(deployment, allocatableGPU)
				deviceThroughput := getIntervalThroughput(name, scaleUpDevice, redisThroughput)
				desiredReplicas := getScaleUpDesiredReplicas(deployment[scaleUpDevice], modelThroughput, deviceThroughput, queueLength, allocatableGPU)
				s.Scale(ctx, deployment[scaleUpDevice], int32(desiredReplicas))
			} else {
				// ScaleDown
				scaleDownDevice := getScaleDownDevice(deployment)
				deviceThroughput := getIntervalThroughput(name, scaleDownDevice, redisThroughput)
				desiredReplicas := getScaleDownDesiredReplicas(deployment[scaleDownDevice], modelThroughput, deviceThroughput, queueLength)
				s.Scale(ctx, deployment[scaleDownDevice], int32(desiredReplicas))
			}
		}
		time.Sleep(time.Second * time.Duration(SleepInterval))
	}
}

func getScaleUpDevice(deployment map[string]*appsv1.Deployment, allocatableGPU int) string {
	if _, ok := deployment["gpu"]; ok && 0 < allocatableGPU {
		return GPU
	}
	return CPU
}

func getScaleDownDevice(deployment map[string]*appsv1.Deployment) string {
	if _, ok := deployment["gpu"]; ok && 0 < *deployment["gpu"].Spec.Replicas {
		return GPU
	}
	return CPU
}

func getIntervalThroughput(name string, device string, redisThroughput map[string]float64) float64 {
	key := fmt.Sprintf("%s-%s-throughput", name, device)
	if _, ok := redisThroughput[key]; ok {
		return redisThroughput[key] * float64(SleepInterval)
	}
	return 0
}

func getScaleUpDesiredReplicas(deployment *appsv1.Deployment, modelThroughput float64, deviceThroughput float64, queueLength int, allocatableGPU int) int {
	// Uninitialized model
	if queueLength != 0 && deviceThroughput == 0 {
		return 1
	}

	device := deployment.Labels["deviceType"]
	currentReplicas := int(*deployment.Spec.Replicas)
	desiredReplicas := currentReplicas + int(math.Ceil((float64(queueLength)-modelThroughput)/deviceThroughput))
	if device == GPU && allocatableGPU < int(desiredReplicas-currentReplicas) {
		desiredReplicas = currentReplicas + allocatableGPU
	}

	if ReplicasLimit < desiredReplicas {
		return ReplicasLimit
	}

	return desiredReplicas
}

func getScaleDownDesiredReplicas(deployment *appsv1.Deployment, modelThroughput float64, deviceThroughput float64, queueLength int) int {
	currentReplicas := int(*deployment.Spec.Replicas)
	desiredReplicas := currentReplicas + int(math.Floor((float64(queueLength)-modelThroughput)/deviceThroughput))
	desiredReplicas = int(math.Max(0, float64(desiredReplicas)))

	// Process remaining tasks
	if queueLength != 0 && desiredReplicas == 0 {
		return 1
	}
	return desiredReplicas
}

func checkStabilizationWindow(deployment *appsv1.Deployment, stabilizationTime int) bool {
	unixTime := int(time.Now().Unix())
	labels := deployment.Labels
	if _, ok := labels["lastScaledTime"]; ok {
		lastScaledTime, _ := strconv.Atoi(labels["lastScaledTime"])
		elapsed_seconds := unixTime - lastScaledTime
		if elapsed_seconds < stabilizationTime {
			return true
		}
	}
	return false
}

func (s *AutoscalerEventSource) Scale(ctx context.Context, deployment *appsv1.Deployment, desiredReplicas int32) error {
	currentReplicas := *deployment.Spec.Replicas
	if currentReplicas == desiredReplicas {
		return nil
	}

	scaleType := "ScaleDown"
	if currentReplicas < desiredReplicas {
		scaleType = "ScaleUp"
	}

	if checkStabilizationWindow(deployment, 0) {
		fmt.Printf("[%s] skipped %s because of stabilization\n", scaleType, deployment.Name)
		return nil
	}

	unixTime := int(time.Now().Unix())
	timestamp := strconv.Itoa(unixTime)
	deployment.Labels["lastScaledTime"] = timestamp
	*deployment.Spec.Replicas = desiredReplicas
	err := s.client.Update(ctx, deployment)
	if err != nil {
		println(err.Error())
	}

	fmt.Printf("[%s] %s to %d replicas\n", scaleType, deployment.Name, *deployment.Spec.Replicas)
	return nil
}

func (s *AutoscalerEventSource) GetDeployments(ctx context.Context) map[string]map[string]*appsv1.Deployment {
	var deploymentList appsv1.DeploymentList
	s.client.List(ctx, &deploymentList)

	targetDeploymentList := make(map[string]map[string]*appsv1.Deployment)
	for _, deployment := range deploymentList.Items {
		if deployment.Labels["app"] == "custom-autoscaler" {
			name := strings.Split(deployment.Name, "-")[0]
			if _, ok := targetDeploymentList[name]; !ok {
				targetDeploymentList[name] = make(map[string]*appsv1.Deployment)
			}
			gpu := deployment.Spec.Template.Spec.Containers[0].Resources.Limits["nvidia.com/gpu"]
			gpu_count, _ := gpu.AsInt64()
			if gpu_count != 0 {
				targetDeploymentList[name][GPU] = deployment.DeepCopy()
				targetDeploymentList[name][GPU].Labels["deviceType"] = GPU
			} else {
				targetDeploymentList[name][CPU] = deployment.DeepCopy()
				targetDeploymentList[name][CPU].Labels["deviceType"] = CPU
			}
		}
	}
	return targetDeploymentList
}

func (s *AutoscalerEventSource) GetAllGPU(ctx context.Context) int {
	var nodeList corev1.NodeList
	s.client.List(ctx, &nodeList)

	sum := 0
	for _, node := range nodeList.Items {
		gpu := node.Status.Allocatable["nvidia.com/gpu"]
		gpu_count, _ := gpu.AsInt64()
		sum += int(gpu_count)
	}
	return sum
}

func (s *AutoscalerEventSource) GetAllocatedGPU(ctx context.Context) int {
	var podList corev1.PodList
	s.client.List(ctx, &podList)

	sum := 0
	for _, pod := range podList.Items {
		gpu := pod.Spec.Containers[0].Resources.Limits["nvidia.com/gpu"]
		gpu_count, _ := gpu.AsInt64()
		sum += int(gpu_count)
	}
	return sum
}

func (s *AutoscalerEventSource) Start(ctx context.Context, eh handler.EventHandler, _ workqueue.RateLimitingInterface, _ ...predicate.Predicate) error {
	// go s.PollingMetric(ctx)
	go s.PollingRedis(ctx)

	return nil
}

func (s *AutoscalerEventSource) WaitForSync(_ context.Context) error {
	return nil
}
