package controller

import (
	"context"

	v1 "github.com/hlee118/custom-autoscaler/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// spec:
//   replicas: 1
//   selector:
//     matchLabels:
//       app: el
//   template:
//     metadata:
//       labels:
//         app: el
//     spec:
//       containers:
//       - name: el
//         image: hlee118/el
//         ports:
//         - containerPort: 80
//         {{- if .Values.env }}
//         env:
//           {{ toYaml $.Values.env | nindent 8 }}
//         {{- end }}
//         volumeMounts:
//         - name: hostpath
//           mountPath: /root/.cache
//         resources:
//           limits:
//             nvidia.com/gpu: "1"
//       volumes:
//       - name: hostpath
//         hostPath:
//           path: /root/.cache
//           type: Directory

type InferenceEventHandler struct {
	client client.Client
}

func NewInferenceEventHandler() (*InferenceEventHandler, error) {
	return &InferenceEventHandler{}, nil
}

func (h *InferenceEventHandler) Create(c context.Context, e event.CreateEvent, _ workqueue.RateLimitingInterface) {
	var replicas = int32(3)
	deployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "el-cpu-deployment",
			Labels: map[string]string{
				"app": "custom-autoscaler",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "el",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "el",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "el",
							Image: "image: hlee118/el",
							Ports: []corev1.ContainerPort{
								{ContainerPort: 80},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "REDIS_HOST",
									Value: RedisHost(),
								},
								{
									Name: "REDIS_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: RedisSecretName(),
											},
											Key: RedisSecretPasswordKey(),
										},
									},
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "hostpath",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/root/.cache",
								},
							},
						},
					},
				},
			},
		},
	}

	autoscaler := v1.Autoscaler{}
	h.client.Create(c, &deployment)
	h.client.Create(c, &autoscaler)
}

func (h *InferenceEventHandler) Update(c context.Context, e event.UpdateEvent, _ workqueue.RateLimitingInterface) {
}

func (h *InferenceEventHandler) Delete(c context.Context, e event.DeleteEvent, _ workqueue.RateLimitingInterface) {
}

func (h *InferenceEventHandler) Generic(c context.Context, _ event.GenericEvent, _ workqueue.RateLimitingInterface) {
}
