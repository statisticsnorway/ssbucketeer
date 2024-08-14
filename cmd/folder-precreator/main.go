package main

import (
	"context"
	"os"
	"path"

	"cloud.google.com/go/storage"
	"golang.org/x/exp/maps"
	"google.golang.org/api/iterator"
)

func main() {
	ctx := context.Background()

	storage, err := storage.NewClient(ctx)
	if err != nil {
		panic(err)
	}

	pc := Precreator{storage: storage}

	err = pc.PopulateAllBucketFolders(ctx, "/buckets")
	if err != nil {
		panic(err)
	}
}

type Precreator struct {
	storage *storage.Client
}

type BucketMount struct {
	BucketName string
	MountPoint string
}

func (c *Precreator) PopulateAllBucketFolders(ctx context.Context, baseMountPoint string) error {
	dirEntries, err := os.ReadDir(baseMountPoint)
	if err != nil {
		return err
	}

	for _, entry := range dirEntries {
		if entry.IsDir() {
			if err := c.PopulateBucketFolders(ctx, BucketMount{
				BucketName: entry.Name(),
				MountPoint: path.Join(baseMountPoint, entry.Name()),
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *Precreator) PopulateBucketFolders(ctx context.Context, bm BucketMount) error {
	folders, err := c.ListBucketFolders(ctx, bm.BucketName)
	if err != nil {
		return err
	}

	return c.CreateFolders(bm.MountPoint, folders)
}

func (c *Precreator) CreateFolders(prefixPath string, folders []string) error {
	for _, folder := range folders {
		if err := os.MkdirAll(path.Join(prefixPath, folder), 0770); err != nil {
			return err
		}
	}
	return nil
}

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
