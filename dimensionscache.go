package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/golang/groupcache"
	"net/http"
	"net/url"
)

type DimensionsCache struct {
	group *groupcache.Group
}

func NewDimensionsCache() *DimensionsCache {
	g := groupcache.NewGroup("DimensionsCache", 64<<18, groupcache.GetterFunc(
		func(ctx groupcache.Context, key string, dest groupcache.Sink) error {
			fmt.Printf("Calculating Dimensions for %s\n", key)
			u, err := url.Parse(key)
			if err != nil {
				return err
			}

			result := calculateDimensions(u)
			var buffer = bytes.Buffer{}
			binary.Write(&buffer, binary.BigEndian, result)
			dest.SetBytes(buffer.Bytes())

			fmt.Println("Filled Cache")
			return nil
		}))
	dc := DimensionsCache{g}
	return &dc
}

func (c *DimensionsCache) Get(url *url.URL) (d Dimensions, err error) {
	var data []byte
	c.group.Get(nil, url.String(), groupcache.AllocatingByteSliceSink(&data))

	d = Dimensions{}
	err = binary.Read(bytes.NewReader(data), binary.BigEndian, &d)
	return d, err
}

func (c *DimensionsCache) Start(listenUrl string) {
	peers := groupcache.NewHTTPPool(listenUrl)
	peers.Set(listenUrl)
	http.ListenAndServe("http://localhost:8001", http.HandlerFunc(peers.ServeHTTP))
}
