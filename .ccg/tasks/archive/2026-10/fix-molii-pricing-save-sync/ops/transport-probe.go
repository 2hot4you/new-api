// Temporary operational probe: fixed-origin read-only transport, no database writes.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/internal/catalogtransport"
)

func main() {
	snapshot, err := catalogtransport.Fetch(context.Background(), os.Getenv("CATALOG_SYNC_TOKEN"), "dev")
	if err != nil {
		fmt.Fprintln(os.Stderr, "fixed-origin catalog export verification failed; credentials and response withheld")
		os.Exit(1)
	}
	result, err := common.Marshal(map[string]any{"source_id": snapshot.SourceID, "validated": true})
	if err != nil {
		os.Exit(1)
	}
	fmt.Println(string(result))
}
