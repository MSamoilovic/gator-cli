package main

import (
	"context"
	"fmt"
)

func handlerAgg(_ *state, _ command) error {
	feed, err := fetchFeed(context.Background(), "https://www.wagslane.dev/index.xml")
	if err != nil {
		return fmt.Errorf("error fetching feed: %v", err)
	}

	fmt.Printf("%+v\n", feed)
	return nil
}