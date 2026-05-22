//go:build glyph_story

package main

import (
	"fmt"

	diffview "github.com/truffle-dev/glyph/components/diff-view"
	"github.com/truffle-dev/glyph/components/theme"
)

const sampleSmall = `--- a/main.go
+++ b/main.go
@@ -1,4 +1,5 @@
 package main

-import "fmt"
+import "os"
+import "log"

 func main() {
`

const sampleLarger = `--- a/server.go
+++ b/server.go
@@ -1,8 +1,12 @@
 package server

 import (
+	"context"
 	"net/http"
+	"time"
 )

-func Run(addr string) error {
-	return http.ListenAndServe(addr, nil)
+func Run(ctx context.Context, addr string) error {
+	srv := &http.Server{Addr: addr, ReadHeaderTimeout: 5 * time.Second}
+	go func() { <-ctx.Done(); srv.Close() }()
+	return srv.ListenAndServe()
 }
`

func main() {
	small := diffview.New(theme.Default).WithSize(70, 10).WithLines(diffview.ParseUnified(sampleSmall))
	larger := diffview.New(theme.Default).WithSize(72, 18).WithLines(diffview.ParseUnified(sampleLarger))
	noNums := diffview.New(theme.Default).WithSize(72, 18).WithLineNumbers(false).WithLines(diffview.ParseUnified(sampleLarger))

	for _, s := range []struct {
		name string
		v    diffview.View
	}{
		{"small diff", small},
		{"larger diff (with line numbers)", larger},
		{"larger diff (no line numbers)", noNums},
	} {
		fmt.Println(s.name)
		fmt.Println(s.v.View())
		fmt.Println()
	}
}
