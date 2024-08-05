package main

import (
	"context"
	"fmt"

	rm "cloud.google.com/go/resourcemanager/apiv3"
	rmpb "cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
)

func main() {

	a := "ABCDEF"
	b := a[0]
	fmt.Println(string(b))
	fmt.Println(a == "ABCDEF")

	ctx := context.Background()
	folders, err := rm.NewFoldersClient(ctx)
	if err != nil {
		panic(err)
	}

	teamFolderIt := folders.SearchFolders(ctx, &rmpb.SearchFoldersRequest{
		Query: fmt.Sprintf(`parent=folders/%s AND state=ACTIVE AND displayName="%s"`, "618856009812", "dapla-skyinfra"),
	})

	folder, err := teamFolderIt.Next()
	if err != nil {
		panic(err)
	}

	fmt.Println(*folder)
}
