package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"

	"cloud.google.com/go/storage"
	"golang.org/x/exp/maps"
	"google.golang.org/api/iterator"
)

func main() {
	ctx := context.Background()

	// Panic if we cannot create a storage client as it is essential
	storage, err := storage.NewClient(ctx)
	if err != nil {
		panic(err)
	}

	pc := Precreator{storage: storage}

	// Errors from this function aren't necessarily catastrophic..
	// Some folders in GCS might share a name with a file in the same
	// folder, as blob storage does not really have folders..
	err = pc.PopulateAllBucketFolders(ctx, "/buckets")
	if err != nil {
		fmt.Printf("populate bucket folders: %v", err)
	}
}

// Precreator contains functionality for creating an identical folder structure
// locally as what is found in a GCS bucket.
type Precreator struct {
	storage *storage.Client
}

// BucketMount describes where bucket BucketName is mounted (MountPoint)
// in the local filesystem
type BucketMount struct {
	BucketName string
	MountPoint string
}

// PopulateAllBucketFolders looks at all the mounted bucket folders,
// and populates them with the folders present in the GCS bucket.
func (c *Precreator) PopulateAllBucketFolders(ctx context.Context, baseMountPoint string) error {
	dirEntries, err := os.ReadDir(baseMountPoint)
	if err != nil {
		return err
	}

	var allErr error = nil
	for _, entry := range dirEntries {
		if entry.IsDir() {
			if err := c.PopulateBucketFolders(ctx, BucketMount{
				BucketName: entry.Name(),
				MountPoint: path.Join(baseMountPoint, entry.Name()),
			}); err != nil {
				allErr = errors.Join(allErr, err)
			}
		}
	}

	return allErr
}

// PopulateBucketFolders populates a single bucket folder with the folders it should have
func (c *Precreator) PopulateBucketFolders(ctx context.Context, bm BucketMount) error {
	folders, err := c.ListBucketFolders(ctx, bm.BucketName)
	if err != nil {
		return err
	}

	return c.CreateFolders(bm.MountPoint, folders)
}

// CreateFolders creates the folders given in folders under the prefixPath
func (c *Precreator) CreateFolders(prefixPath string, folders []string) error {
	var allErr error = nil
	for _, folder := range folders {
		if err := os.MkdirAll(path.Join(prefixPath, folder), 0770); err != nil {
			allErr = errors.Join(allErr, err)
		}
	}
	return allErr
}

// ListBucketFolders lists all folders found in the given GCS bucket
func (c *Precreator) ListBucketFolders(ctx context.Context, bucket string) ([]string, error) {
	b := c.storage.Bucket(bucket)
	it := b.Objects(ctx, &storage.Query{Prefix: ""})

	names := make(map[string]struct{})

	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		names[path.Dir(attrs.Name)] = struct{}{}
	}

	return maps.Keys(names), nil
}
