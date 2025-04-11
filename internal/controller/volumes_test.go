package controller

import (
	"testing"

	"github.com/go-test/deep"
	corev1 "k8s.io/api/core/v1"
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
			Volume:      corev1.Volume{Name: "my-volume"},
			Name:        "abc",
			Expected:    false,
		},
		{
			Description: "Should return true if name matches",
			Volume:      corev1.Volume{Name: "my-volume"},
			Name:        "my-volume",
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
			VolumeMount: corev1.VolumeMount{Name: "my-volume"},
			Name:        "abc",
			Expected:    false,
		},
		{
			Description: "Should return true if name matches",
			VolumeMount: corev1.VolumeMount{Name: "my-volume"},
			Name:        "my-volume",
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
						Name: "not-gcsfuse",
					},
				},
				Containers: []corev1.Container{
					{
						Name: "not-gcsfuse",
						VolumeMounts: []corev1.VolumeMount{
							{
								Name: "not-gcsfuse",
							},
						},
					},
				},
			},
			ExpectedPodSpec: corev1.PodSpec{
				Volumes: []corev1.Volume{
					{
						Name: "not-gcsfuse",
					},
				},
				Containers: []corev1.Container{
					{
						Name: "not-gcsfuse",
						VolumeMounts: []corev1.VolumeMount{
							{
								Name: "not-gcsfuse",
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
						Name: "not-gcsfuse",
					},
					{
						Name: "gcsfuse-2",
					},
				},
				Containers: []corev1.Container{
					{
						Name: "not-gcsfuse",
						VolumeMounts: []corev1.VolumeMount{
							{
								Name: "gcsfuse-1",
							},
							{
								Name: "not-gcsfuse",
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
						Name: "not-gcsfuse",
					},
				},
				Containers: []corev1.Container{
					{
						Name: "not-gcsfuse",
						VolumeMounts: []corev1.VolumeMount{
							{
								Name: "not-gcsfuse",
							},
						},
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
