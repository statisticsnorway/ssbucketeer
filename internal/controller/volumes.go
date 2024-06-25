package controller

import (
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
)

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

func addBucketsToPodSpec(podspec *corev1.PodSpec, container *corev1.Container, bucketNames []string) (shouldUpdate bool) {
	volumes := make([]corev1.Volume, 0, len(bucketNames))
	volumeMounts := make([]corev1.VolumeMount, 0, len(bucketNames))
	for _, bucket := range bucketNames {
		volumeName := fmt.Sprintf("gcsfuse-%s", bucket)

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
				MountPath: fmt.Sprintf("/buckets/%s", bucket),
				ReadOnly:  false,
			})
		}
	}

	if len(volumes) > 0 {
		podspec.Volumes = append(podspec.Volumes, volumes...)
	}
	if len(volumeMounts) > 0 {
		container.VolumeMounts = append(container.VolumeMounts, volumeMounts...)
	}

	return len(volumes) > 0 || len(volumeMounts) > 0
}
