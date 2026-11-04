// Command contract says how much of the client's contract (api/openapi.json)
// this server answers: each operation built, and each still answering 501.
//
//	go run ./cmd/contract          the counts, then what is left by tag
//	go run ./cmd/contract -all     every operation, built or not
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"

	"github.com/evrone/go-clean-template/api"
	"github.com/evrone/go-clean-template/internal/controller/restapi/clientapi"
	"github.com/evrone/go-clean-template/pkg/contract"
	"github.com/gofiber/fiber/v3"
)

func main() {
	all := flag.Bool("all", false, "list every operation")

	flag.Parse()

	spec, err := contract.Load(api.Spec())
	if err != nil {
		log.Fatal(err)
	}

	a, err := clientapi.NewRoutes(fiber.New(), &clientapi.Deps{Contract: spec})
	if err != nil {
		log.Fatal(err)
	}

	built := a.Built()
	ops := spec.Operations()

	fmt.Fprintf(os.Stdout, "contract: %d of %d operations built, %d answer 501\n", len(built), len(ops), len(ops)-len(built))

	for _, op := range ops {
		done := slices.Contains(built, op.ID)
		if done && !*all {
			continue
		}

		mark := "  "
		if done {
			mark = "✓ "
		}

		fmt.Fprintf(os.Stdout, "%s%-7s %-40s %s (%s)\n", mark, op.Method, op.Path, op.ID, strings.Join(op.Permission, ", "))
	}
}
