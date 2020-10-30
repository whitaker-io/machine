package kubernetes

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/spf13/viper"
	"github.com/whitaker-io/machine"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	// enabling gcp auth
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

// Terminus func for providing a terminus that runs a k8s job with the payload as a base64 encoded json env var called PAYLOAD
func Terminus(v *viper.Viper) machine.Terminus {
	name := v.GetString("name")
	namespace := v.GetString("namespace")
	inCluster := v.GetBool("inCluster")
	labels := v.GetStringMapString("labels")

	clientset := client(inCluster)

	return func(m []map[string]interface{}) error {
		payload, err := json.Marshal(m)

		if err != nil {
			return err
		}

		id := uuid.New().String()

		_, err = clientset.BatchV1().Jobs(namespace).Create(context.Background(), &batchv1.Job{
			Spec: batchv1.JobSpec{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name + "-" + id,
						Namespace: namespace,
						Labels:    labels,
					},
					Spec: spec(v, payload),
				},
			},
		}, metav1.CreateOptions{})

		return err
	}
}

func spec(v *viper.Viper, payload []byte) corev1.PodSpec {
	name := v.GetString("name")
	namespace := v.GetString("namespace")
	image := v.GetString("image")
	command := v.GetStringSlice("command")
	args := v.GetStringSlice("args")
	environment := v.GetStringMapString("environment")
	deadline := v.GetInt64("deadline")
	privileged := v.GetBool("privileged")
	limitCPU := v.GetString("limits.cpu")
	limitMemory := v.GetString("limits.memory")
	requestCPU := v.GetString("requests.cpu")
	requestMemory := v.GetString("requests.memory")

	vars := []corev1.EnvVar{
		{Name: "NAMESPACE", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"}}},
		{Name: "NODE_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"}}},
		{Name: "HOST_IP", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.hostIP"}}},
		{Name: "POD_IP", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.podIP"}}},
		{Name: "NAME", Value: name},
		{Name: "PAYLOAD", Value: base64.StdEncoding.EncodeToString(payload)},
	}

	for k, v := range environment {
		vars = append(vars, corev1.EnvVar{Name: k, Value: v})
	}

	limits := corev1.ResourceList{
		corev1.ResourceName("cpu"):    resource.Quantity{Format: resource.Format("2000m")},
		corev1.ResourceName("memory"): resource.Quantity{Format: resource.Format("2000Mi")},
	}

	if limitCPU == "" {
		limits = corev1.ResourceList{
			corev1.ResourceName("cpu"):    resource.Quantity{Format: resource.Format(limitCPU)},
			corev1.ResourceName("memory"): resource.Quantity{Format: resource.Format(limitMemory)},
		}
	}

	requests := corev1.ResourceList{
		corev1.ResourceName("cpu"):    resource.Quantity{Format: resource.Format("2000m")},
		corev1.ResourceName("memory"): resource.Quantity{Format: resource.Format("2000Mi")},
	}

	if requestCPU == "" {
		requests = corev1.ResourceList{
			corev1.ResourceName("cpu"):    resource.Quantity{Format: resource.Format(requestCPU)},
			corev1.ResourceName("memory"): resource.Quantity{Format: resource.Format(requestMemory)},
		}
	}

	return corev1.PodSpec{
		Affinity: &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{
					{
						Weight: 100,
						Preference: corev1.NodeSelectorTerm{
							MatchFields: []corev1.NodeSelectorRequirement{
								{Key: "preemptible", Operator: corev1.NodeSelectorOpExists},
							},
						},
					},
				},
			},
			PodAntiAffinity: &corev1.PodAntiAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
					{
						Weight: 100,
						PodAffinityTerm: corev1.PodAffinityTerm{
							TopologyKey:   "kubernetes.io/hostname",
							LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"namespace": namespace, "app": name}},
						},
					},
					{
						Weight: 99,
						PodAffinityTerm: corev1.PodAffinityTerm{
							TopologyKey:   "failure-domain.beta.kubernetes.io/zone",
							LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"namespace": namespace, "app": name}},
						},
					},
				},
			},
		},
		ActiveDeadlineSeconds: &deadline,
		Containers: []corev1.Container{
			{
				Image:           image,
				ImagePullPolicy: corev1.PullAlways,
				Env:             vars,
				Command:         command,
				Args:            args,
				Resources:       corev1.ResourceRequirements{Limits: limits, Requests: requests},
				SecurityContext: &corev1.SecurityContext{Privileged: &privileged},
			},
		},
	}
}

func client(inCluster bool) *kubernetes.Clientset {
	if inCluster {
		// creates the in-cluster config
		config, err := rest.InClusterConfig()
		if err != nil {
			panic(err.Error())
		}
		// creates the clientset
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			panic(err.Error())
		}

		return clientset
	}

	var kubeconfig *string
	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)

	if err != nil {
		panic(err.Error())
	}

	return clientset
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}
