package controller

import (
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
)

func addOrUpdatePrecreator(podspec *corev1.PodSpec, bucketMounts map[string]string, precreatorImage string) (modified bool) {
	precreator := corev1.Container{
		Image: precreatorImage,
		Name:  precreatorContainerName,
	}
	volumeMounts := make(map[string]corev1.VolumeMount, len(bucketMounts))
	for mountPoint, bucket := range bucketMounts {
		volumeName := maxLengthVolumeName(mountPoint)
		volumeMounts[volumeName] = corev1.VolumeMount{
			Name:      volumeName,
			MountPath: fmt.Sprintf("/buckets/%s", bucket),
		}
	}
	for _, mount := range volumeMounts {
		precreator.VolumeMounts = append(precreator.VolumeMounts, mount)
	}
	for i, c := range podspec.Containers {
		if c.Name == precreator.Name {
			if c.Image != precreatorImage ||
				!slices.EqualFunc(precreator.VolumeMounts, c.VolumeMounts, func(a, b corev1.VolumeMount) bool {
					return a.Name == b.Name
				}) {
				podspec.Containers[i] = precreator
				return true
			}
			return false
		}
	}

	podspec.Containers = append(podspec.Containers, precreator)
	return true
}

func removePrecreator(podspec *corev1.PodSpec) (modified bool) {
	podspec.Containers = slices.DeleteFunc(podspec.Containers, func(c corev1.Container) bool {
		if c.Name == precreatorContainerName {
			modified = true
			return true
		}
		return false
	})
	return false
}
