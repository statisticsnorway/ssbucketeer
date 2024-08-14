package controller

import (
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
)

// ptr is a convenience function for generating pointers of primitive types
func ptr[T any](t T) *T {
	return &t
}

func volumeNameIs(name string) func(v corev1.Volume) bool {
	return func(v corev1.Volume) bool {
		return v.Name == name
	}
}

func volumeMountNameIs(name string) func(v corev1.VolumeMount) bool {
	return func(v corev1.VolumeMount) bool {
		return v.Name == name
	}
}

func addBucketsToPodSpec(podspec *corev1.PodSpec, container *corev1.Container, bucketMounts map[string]string, precreatorImage string) (shouldUpdate bool) {
	volumes := make([]corev1.Volume, 0, len(bucketMounts))
	volumeMounts := make([]corev1.VolumeMount, 0, len(bucketMounts))
	for mountPoint, bucket := range bucketMounts {
		volumeName := fmt.Sprintf("gcsfuse-%s", mountPoint)

		// TODO: Test whether the group has read/write access
		if !slices.ContainsFunc(podspec.Volumes, volumeNameIs(volumeName)) {
			volumes = append(volumes, corev1.Volume{
				Name: volumeName,
				VolumeSource: corev1.VolumeSource{
					CSI: &corev1.CSIVolumeSource{
						Driver:   "gcsfuse.csi.storage.gke.io",
						ReadOnly: ptr(false),
						VolumeAttributes: map[string]string{
							"bucketName":             bucket,
							"mountOptions":           "uid=1000,gid=100",
							"gcsfuseLoggingSeverity": "warning",
						},
					},
				},
			})
		}

		if !slices.ContainsFunc(container.VolumeMounts, volumeMountNameIs(volumeName)) {
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      volumeName,
				MountPath: fmt.Sprintf("/buckets/%s", mountPoint),
				ReadOnly:  false,
			})
		}
	}

	precreator := corev1.Container{
		Image: precreatorImage,
		Name:  "bucket-folders-precreator",
	}
	for mountPoint, bucket := range bucketMounts {
		volumeName := fmt.Sprintf("gcsfuse-%s", mountPoint)
		precreator.VolumeMounts = append(precreator.VolumeMounts, corev1.VolumeMount{
			Name:      volumeName,
			MountPath: fmt.Sprintf("/buckets/%s", bucket),
		})
	}
	if !slices.ContainsFunc(podspec.InitContainers, func(c corev1.Container) bool {
		return c.Name == precreator.Name
	}) {
		podspec.InitContainers = append(podspec.InitContainers, precreator)
	}

	if len(volumes) > 0 {
		podspec.Volumes = append(podspec.Volumes, volumes...)
	}
	if len(volumeMounts) > 0 {
		container.VolumeMounts = append(container.VolumeMounts, volumeMounts...)
	}

	return len(volumes) > 0 || len(volumeMounts) > 0
}
