package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/quixote-liu/docs/internal/knowledge"
)

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	global := flag.NewFlagSet("knowledge", flag.ContinueOnError)
	global.SetOutput(stderr)
	defaultRoot := os.Getenv("KNOWLEDGE_ROOT")
	if defaultRoot == "" {
		defaultRoot = "."
	}
	root := global.String("root", defaultRoot, "knowledge directory")
	jsonOutput := global.Bool("json", false, "emit machine-readable JSON")
	global.Usage = func() { printUsage(stderr) }
	if err := global.Parse(args); err != nil {
		return 2
	}
	remaining := global.Args()
	if len(remaining) == 0 {
		printUsage(stderr)
		return 2
	}

	store, err := knowledge.NewStore(*root)
	if err != nil {
		return fail(stderr, *jsonOutput, err)
	}
	command, commandArgs := remaining[0], remaining[1:]
	switch command {
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	case "init":
		if len(commandArgs) != 0 {
			return fail(stderr, *jsonOutput, errors.New("init takes no arguments"))
		}
		if err := store.Init(); err != nil {
			return fail(stderr, *jsonOutput, err)
		}
		return emit(stdout, *jsonOutput, map[string]string{"root": store.Root()}, store.Root()+"\n")
	case "categories":
		if len(commandArgs) != 0 {
			return fail(stderr, *jsonOutput, errors.New("categories takes no arguments"))
		}
		items, err := store.Categories()
		if err != nil {
			return fail(stderr, *jsonOutput, err)
		}
		return emit(stdout, *jsonOutput, items, lines(items))
	case "list":
		flags := newFlags("list", stderr)
		category := flags.String("category", "", "only list this category")
		if err := flags.Parse(commandArgs); err != nil || flags.NArg() != 0 {
			if err == nil {
				err = errors.New("list takes no positional arguments")
			}
			return fail(stderr, *jsonOutput, err)
		}
		docs, err := store.List(*category)
		if err != nil {
			return fail(stderr, *jsonOutput, err)
		}
		var text strings.Builder
		for _, doc := range docs {
			fmt.Fprintf(&text, "%s\t%s\n", doc.Path, doc.Title)
		}
		return emit(stdout, *jsonOutput, docs, text.String())
	case "read":
		if len(commandArgs) != 1 {
			return fail(stderr, *jsonOutput, errors.New("usage: knowledge [global flags] read <path>"))
		}
		doc, err := store.Read(commandArgs[0])
		if err != nil {
			return fail(stderr, *jsonOutput, err)
		}
		return emit(stdout, *jsonOutput, doc, doc.Content)
	case "write":
		return runWrite(store, commandArgs, stdin, stdout, stderr, *jsonOutput)
	case "search":
		flags := newFlags("search", stderr)
		category := flags.String("category", "", "only search this category")
		limit := flags.Int("limit", 50, "maximum number of matches; 0 means unlimited")
		if err := flags.Parse(commandArgs); err != nil {
			return 2
		}
		query := strings.Join(flags.Args(), " ")
		matches, err := store.Search(query, *category, *limit)
		if err != nil {
			return fail(stderr, *jsonOutput, err)
		}
		var text strings.Builder
		for _, match := range matches {
			fmt.Fprintf(&text, "%s:%d: %s\n", match.Path, match.Line, match.Text)
		}
		return emit(stdout, *jsonOutput, matches, text.String())
	default:
		return fail(stderr, *jsonOutput, fmt.Errorf("unknown command %q", command))
	}
}

func runWrite(store *knowledge.Store, args []string, stdin io.Reader, stdout, stderr io.Writer, jsonOutput bool) int {
	flags := newFlags("write", stderr)
	mode := flags.String("mode", "create", "create, overwrite, or append")
	content := flags.String("content", "", "document content")
	inputFile := flags.String("file", "", "read content from a file")
	stdinInput := flags.Bool("stdin", false, "read content from stdin")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 1 {
		return fail(stderr, jsonOutput, errors.New("usage: knowledge [global flags] write [flags] <path>"))
	}
	sources := 0
	if flags.Lookup("content").Value.String() != "" || flagWasSet(flags, "content") {
		sources++
	}
	if *inputFile != "" {
		sources++
	}
	if *stdinInput {
		sources++
	}
	if sources != 1 {
		return fail(stderr, jsonOutput, errors.New("choose exactly one content source: --content, --file, or --stdin"))
	}
	data := []byte(*content)
	var err error
	if *inputFile != "" {
		data, err = os.ReadFile(*inputFile)
	} else if *stdinInput {
		data, err = io.ReadAll(stdin)
	}
	if err != nil {
		return fail(stderr, jsonOutput, fmt.Errorf("read input: %w", err))
	}
	doc, err := store.Write(flags.Arg(0), string(data), *mode)
	if err != nil {
		return fail(stderr, jsonOutput, err)
	}
	doc.Content = ""
	return emit(stdout, jsonOutput, doc, doc.Path+"\n")
}

func newFlags(name string, output io.Writer) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(output)
	return flags
}

func flagWasSet(flags *flag.FlagSet, name string) bool {
	set := false
	flags.Visit(func(item *flag.Flag) {
		if item.Name == name {
			set = true
		}
	})
	return set
}

func emit(output io.Writer, asJSON bool, value any, text string) int {
	if asJSON {
		encoder := json.NewEncoder(output)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(value); err != nil {
			fmt.Fprintln(output, `{"error":"encode output"}`)
			return 1
		}
		return 0
	}
	_, _ = io.WriteString(output, text)
	return 0
}

func fail(output io.Writer, asJSON bool, err error) int {
	if asJSON {
		_ = json.NewEncoder(output).Encode(map[string]string{"error": err.Error()})
	} else {
		fmt.Fprintln(output, "error:", err)
	}
	return 1
}

func lines(items []string) string {
	if len(items) == 0 {
		return ""
	}
	return strings.Join(items, "\n") + "\n"
}

func printUsage(output io.Writer) {
	fmt.Fprintln(output, `knowledge - a Git-friendly knowledge CLI

Usage:
  knowledge [--root DIR] [--json] <command> [arguments]

Commands:
  init                          Ensure the knowledge root exists
  categories                    List categories containing documents
  list [--category CATEGORY]    List documents
  read <path>                   Read a document
  write [flags] <path>          Create, overwrite, or append a document
  search [flags] <query>        Search document contents

Write flags:
  --content TEXT                Use inline content (including an empty string)
  --file FILE                   Read content from a file
  --stdin                       Read content from standard input
  --mode MODE                   create (default), overwrite, or append

Search flags:
  --category CATEGORY           Search only one category
  --limit N                     Maximum matches (default 50; 0 is unlimited)

Environment:
  KNOWLEDGE_ROOT                Default root (otherwise current directory)`)
}
