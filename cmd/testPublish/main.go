package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
)

func main() {
	for i := 0; i < 1000; i++ {
		payload := []byte(fmt.Sprintf("foobarbaz_%d", i))
		resp, err := http.Post("http://localhost:3000/publish/topic_1", "application/octet-stream", bytes.NewReader(payload))
		if err != nil {

			log.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			return
		}
		fmt.Println(resp)

	}

}
