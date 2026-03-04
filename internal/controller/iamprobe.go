package controller

import (
	"slices"

	corev1 "k8s.io/api/core/v1"
)

func addOrUpdateIamProbe(podspec *corev1.PodSpec, iamProbeImage string) (modified bool) {
	for i, c := range podspec.InitContainers {
		if c.Name == iamProbeContainerName {
			if c.Image != iamProbeImage {
				podspec.Containers[i].Image = iamProbeImage
				return true
			}
			return false
		}
	}

	podspec.InitContainers = append([]corev1.Container{{
		Image: iamProbeImage,
		Name:  iamProbeContainerName,
	}}, podspec.InitContainers...,
	)
	return true
}

func removeIamProbe(podspec *corev1.PodSpec) (modified bool) {
	podspec.InitContainers = slices.DeleteFunc(podspec.InitContainers, func(c corev1.Container) bool {
		if c.Name == iamProbeContainerName {
			modified = true
			return true
		}
		return false
	})
	return false
}
