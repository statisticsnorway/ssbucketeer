package controller

import (
	"testing"

	"github.com/go-test/deep"
	corev1 "k8s.io/api/core/v1"
)

const (
	volumeName     = "my-volume"
	notGcsFuseName = "not-gcsfuse"
)

func TestVolumeNameIs(t *testing.T) {
	type TestCase struct {
		Description string
		Volume      corev1.Volume
		Name        string
		Expected    bool
	}

	tests := []TestCase{
		{
			Description: "Should return false if name does not match",
			Volume:      corev1.Volume{Name: volumeName},
			Name:        "abc",
			Expected:    false,
		},
		{
			Description: "Should return true if name matches",
			Volume:      corev1.Volume{Name: volumeName},
			Name:        volumeName,
			Expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.Description, func(t *testing.T) {
			if res := volumeNameIs(tt.Name)(tt.Volume); res != tt.Expected {
				t.Errorf("volumeNameIs=%v, expected=%v", res, tt.Expected)
			}
		})
	}
}

func TestVolumeMountNameIs(t *testing.T) {
	type TestCase struct {
		Description string
		VolumeMount corev1.VolumeMount
		Name        string
		Expected    bool
	}

	tests := []TestCase{
		{
			Description: "Should return false if name does not match",
			VolumeMount: corev1.VolumeMount{Name: volumeName},
			Name:        "abc",
			Expected:    false,
		},
		{
			Description: "Should return true if name matches",
			VolumeMount: corev1.VolumeMount{Name: volumeName},
			Name:        volumeName,
			Expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.Description, func(t *testing.T) {
			if res := volumeMountNameIs(tt.Name)(tt.VolumeMount); res != tt.Expected {
				t.Errorf("volumeMountNameIs=%v, expected=%v", res, tt.Expected)
			}
		})
	}
}

func TestRemoveBucketsFromPodSpec(t *testing.T) {
	type TestCase struct {
		Description      string
		PodSpec          corev1.PodSpec
		ExpectedPodSpec  corev1.PodSpec
		ExpectedModified bool
	}

	tests := []TestCase{
		{
			Description: "Should not change anything if no gcsfuse volumes/sidecars",
			PodSpec: corev1.PodSpec{
				Volumes: []corev1.Volume{
					{
						Name: notGcsFuseName,
					},
				},
				Containers: []corev1.Container{
					{
						Name: notGcsFuseName,
						VolumeMounts: []corev1.VolumeMount{
							{
								Name: notGcsFuseName,
							},
						},
					},
				},
			},
			ExpectedPodSpec: corev1.PodSpec{
				Volumes: []corev1.Volume{
					{
						Name: notGcsFuseName,
					},
				},
				Containers: []corev1.Container{
					{
						Name: notGcsFuseName,
						VolumeMounts: []corev1.VolumeMount{
							{
								Name: notGcsFuseName,
							},
						},
					},
				},
			},
			ExpectedModified: false,
		},
		{
			Description: "Should remove only relevant volumes and containers",
			PodSpec: corev1.PodSpec{
				Volumes: []corev1.Volume{
					{
						Name: "gcsfuse-1",
					},
					{
						Name: notGcsFuseName,
					},
					{
						Name: "gcsfuse-2",
					},
				},
				Containers: []corev1.Container{
					{
						Name: notGcsFuseName,
						VolumeMounts: []corev1.VolumeMount{
							{
								Name: "gcsfuse-1",
							},
							{
								Name: notGcsFuseName,
							},
							{
								Name: "gcsfuse-2",
							},
						},
					},
					{
						Name: precreatorContainerName,
					},
				},
			},
			ExpectedPodSpec: corev1.PodSpec{
				Volumes: []corev1.Volume{
					{
						Name: notGcsFuseName,
					},
				},
				Containers: []corev1.Container{
					{
						Name: notGcsFuseName,
						VolumeMounts: []corev1.VolumeMount{
							{
								Name: notGcsFuseName,
							},
						},
					},
					{
						Name: precreatorContainerName,
					},
				},
			},
			ExpectedModified: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.Description, func(t *testing.T) {
			if modified := removeBucketsFromPodSpec(&tt.PodSpec, &tt.PodSpec.Containers[0]); modified != tt.ExpectedModified {
				t.Errorf("modified=%v, expected=%v", modified, tt.ExpectedModified)
			}
			if diff := deep.Equal(tt.PodSpec, tt.ExpectedPodSpec); diff != nil {
				t.Errorf("modified PodSpec does not equal expected: %v", diff)
			}
		})
	}

}

func TestMaxLengthVolumeName(t *testing.T) {
	type TestCase struct {
		Description        string
		MountPoint         string
		ExpectedLength     int
		ExpectedVolumeName string
	}
	tests := []TestCase{
		{
			Description:        "Mountpoint shorter than maxLength",
			MountPoint:         "short/team/bucket",
			ExpectedLength:     27,
			ExpectedVolumeName: "gcsfuse-short--team--bucket",
		},
		{
			Description:        "Mountpoint exact maxLength",
			MountPoint:         "exact/team/bucket-scpaxhfblvomjeoixplhtvktqm",
			ExpectedLength:     54,
			ExpectedVolumeName: "gcsfuse-exact--team--bucket-scpaxhfblvomjeoixplhtvktqm",
		},
		{
			Description:        "Mountpoint longer maxLength",
			MountPoint:         "longer/team/bucket-ayjtigfrcojhjkxgyjateuqddmguse",
			ExpectedLength:     54,
			ExpectedVolumeName: "gcsfuse-longer--team--bucket-ayjtigfrcojhjkxg-01f50b1b",
		},
	}
	for _, tt := range tests {
		t.Run(tt.Description, func(t *testing.T) {
			if res := maxLengthVolumeName(tt.MountPoint); len(res) != tt.ExpectedLength {
				t.Errorf("length=%v, expected=%v", len(res), tt.ExpectedLength)
			}
			if res := maxLengthVolumeName(tt.MountPoint); res != tt.ExpectedVolumeName {
				t.Errorf("maxLengthVolumeName=%v, expected=%v", res, tt.ExpectedVolumeName)
			}
		})
	}
}
