package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"k8s.io/api/admission/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/yaml"
)

var (
	globalConfig          Config
	universalDeserializer = serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()
)

type Config struct {
	HTTPConfig   HTTPConfig       `json:"http"`
	Volumes      []v1.Volume      `json:"volumes"`
	VolumeMounts []v1.VolumeMount `json:"volumeMounts"`
}

type HTTPConfig struct {
	ListenAddr string    `json:"listenAddress"`
	TLSConfig  TLSConfig `json:"tls"`
}

type TLSConfig struct {
	TLSCertificateFile string `json:"certfile"`
	TLSKeyFile         string `json:"keyfile"`
}

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func getConfig() (Config, error) {
	config := Config{
		HTTPConfig: HTTPConfig{
			ListenAddr: ":8080",
		},
	}
	config_file := flag.String("config", "", "Configuration path file")
	flag.Parse()

	// Check parameters
	if len(*config_file) == 0 {
		return config, errors.New("config file (-config flag) not specified")
	}
	yamlFile, err := os.Open(*config_file)
	if err != nil {
		return config, err
	}
	err = yaml.NewYAMLOrJSONDecoder(bufio.NewReader(yamlFile), 100).Decode(&config)
	if err != nil {
		return config, err
	}
	return config, nil
}

func main() {
	config, err := getConfig()
	if err != nil {
		log.Fatal(err)
	}
	globalConfig = config
	http.HandleFunc("/health", HandleHealth)
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/mutate", HandleMutate)
	log.Infof("Listening on %v...", config.HTTPConfig.ListenAddr)
	log.Fatal(http.ListenAndServeTLS(config.HTTPConfig.ListenAddr, config.HTTPConfig.TLSConfig.TLSCertificateFile, config.HTTPConfig.TLSConfig.TLSKeyFile, nil))
}

func HandleHealth(w http.ResponseWriter, r *http.Request) {
	log.Info("Health: OK")
	fmt.Fprintf(w, "OK")
}
func HandleMutate(w http.ResponseWriter, r *http.Request) {
	log.Info("New mutate request")
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Error(err)
		return
	}

	var admissionReviewReq v1beta1.AdmissionReview
	if _, _, err := universalDeserializer.Decode(body, nil, &admissionReviewReq); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Errorf("could not deserialize request: %v", err)
		return
	} else if admissionReviewReq.Request == nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Error("malformed admission review: request is nil")
		return
	}

	var pod v1.Pod
	err = json.Unmarshal(admissionReviewReq.Request.Object.Raw, &pod)
	if err != nil {
		log.Errorf("could not unmarshal pod on admission request: %v", err)
		return
	}
	log.Infof("Processing Pod %v (Namespace=%v)...", pod.ObjectMeta.Name, pod.ObjectMeta.Namespace)

	podVolumes := pod.Spec.Volumes
	//volumeType := v1.HostPathDirectory
	//newVolumes := []v1.Volume{
	//	{
	//		Name: "etc-ssl-certs",
	//		VolumeSource: v1.VolumeSource{
	//			HostPath: &v1.HostPathVolumeSource{
	//				Path: "/etc/ssl/certs",
	//				Type: &volumeType,
	//			},
	//		},
	//	},
	//}
	newVolumes := globalConfig.Volumes
	podVolumes = append(podVolumes, newVolumes...)

	//newVolumeMounts := []v1.VolumeMount{
	//	{
	//		Name:      "etc-ssl-certs",
	//		MountPath: "/etc/ssl/certs",
	//		ReadOnly:  true,
	//	},
	//}

	newVolumeMounts := globalConfig.VolumeMounts
	for i := range pod.Spec.Containers {
		pod.Spec.Containers[i].VolumeMounts = append(pod.Spec.Containers[i].VolumeMounts, newVolumeMounts...)
	}

	patches := []patchOperation{
		{
			Op:    "add",
			Path:  "/spec/volumes",
			Value: podVolumes,
		},
		{
			Op:    "replace",
			Path:  "/spec/containers",
			Value: pod.Spec.Containers,
		},
	}

	patchBytes, err := json.Marshal(patches)
	if err != nil {
		log.Errorf("could not marshal pathOperations: %v", err)
		return
	}
	admissionReviewResponse := v1beta1.AdmissionReview{
		Response: &v1beta1.AdmissionResponse{
			UID:     admissionReviewReq.Request.UID,
			Allowed: true,
			Patch:   patchBytes,
		},
	}

	bytes, err := json.Marshal(admissionReviewResponse)
	if err != nil {
		log.Errorf("could not marshal admissionReviewResponse: %v", err)
		return
	}
	log.Infof("Pod %v (Namespace=%v) patched!", pod.ObjectMeta.Name, pod.ObjectMeta.Namespace)
	w.Write(bytes)
}
