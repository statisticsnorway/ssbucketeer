package controller

import (
	"crypto/md5"
	"fmt"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

const (
	precreatorContainerName = "bucket-folders-precreator"
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

func maxLengthVolumeName(mountPoint string) string {
	const maxLength = 54
	const hashLength = 8
	volumeName := fmt.Sprintf("gcsfuse-%s", strings.ReplaceAll(mountPoint, "/", "--"))
	if len(volumeName) > maxLength {
		hash := fmt.Sprintf("%x", md5.Sum([]byte(volumeName)))
		volumeNameHash := fmt.Sprintf("%s-%s", volumeName[:maxLength-hashLength-1], hash[:hashLength])
		return volumeNameHash
	}
	return volumeName
}

func addBucketsToPodSpec(podspec *corev1.PodSpec, container *corev1.Container, bucketMounts map[string]string) (shouldUpdate bool) {
	volumes := make([]corev1.Volume, 0, len(bucketMounts))
	volumeMounts := make([]corev1.VolumeMount, 0, len(bucketMounts))
	for mountPoint, bucket := range bucketMounts {
		volumeName := maxLengthVolumeName(mountPoint)

		// TODO: Test whether the group has read/write access
		if !slices.ContainsFunc(podspec.Volumes, volumeNameIs(volumeName)) {
			volumes = append(volumes, corev1.Volume{
				Name: volumeName,
				VolumeSource: corev1.VolumeSource{
					CSI: &corev1.CSIVolumeSource{
						Driver:   "gcsfuse.csi.storage.gke.io",
						ReadOnly: ptr.To(false),
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

	if len(volumes) > 0 {
		podspec.Volumes = append(podspec.Volumes, volumes...)
	}
	if len(volumeMounts) > 0 {
		container.VolumeMounts = append(container.VolumeMounts, volumeMounts...)
	}

	return len(volumes) > 0 || len(volumeMounts) > 0
}

func removeBucketsFromPodSpec(podspec *corev1.PodSpec, container *corev1.Container) (shouldUpdate bool) {
	modified := false

	podspec.Volumes = slices.DeleteFunc(podspec.Volumes, func(v corev1.Volume) bool {
		if strings.HasPrefix(v.Name, "gcsfuse-") {
			modified = true
			return true
		}
		return false
	})

	container.VolumeMounts = slices.DeleteFunc(container.VolumeMounts, func(m corev1.VolumeMount) bool {
		if strings.HasPrefix(m.Name, "gcsfuse-") {
			modified = true
			return true
		}
		return false
	})

	return modified
}
