package main

/*
#cgo CFLAGS: -I../../libparser
#include <stdlib.h>
#include "parser.h"

// Helper function to call the put callback
static inline void call_put(const struct parser_param *param, int type, char *tag, int lnum, char *file, char *image, void *arg) {
    if (param->put) {
        param->put(type, tag, lnum, file, image, arg);
    }
}

// Helper to call warning
static inline void call_warning(const struct parser_param *param, char *msg) {
    if (param->warning) {
        param->warning("%s", msg);
    }
}
*/
import "C"
import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"unsafe"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// Language aliases map
var languageAliases = map[string]string{
	"fantom":     "fan",
	"haxe":       "haXe",
	"sourcepawn": "sp",
	"typescript": "ts",
	"xbase":      "XBase",
}

// Map from extension (including dot) to language name
var langMap map[string]string

// Tag represents a parsed tag
type Tag struct {
	Type  int
	Tag   string
	Line  int
	File  string
	Image string
}

// ByLine implements sort.Interface for []Tag based on Line number
type ByLine []Tag

func (a ByLine) Len() int           { return len(a) }
func (a ByLine) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByLine) Less(i, j int) bool { return a[i].Line < a[j].Line }

// parseLangMap parses the langmap string into a map
func parseLangMap(s string) {
	if langMap != nil {
		return
	}
	langMap = make(map[string]string)
	mappings := strings.Split(s, ",")
	for _, mapping := range mappings {
		parts := strings.Split(mapping, ":")
		if len(parts) != 2 {
			continue
		}
		lang := parts[0]
		exts := parts[1]
		if len(lang) > 0 && isLower(lang[0]) {
			continue
		}
		for _, ext := range strings.Split(exts, ".") {
			if ext == "" {
				continue
			}
			extKey := "." + ext
			langMap[extKey] = lang
			if runtime.GOOS == "windows" {
				langMap[strings.ToUpper(extKey)] = lang
				langMap[strings.ToLower(extKey)] = lang
			}
		}
	}
}

func isLower(b byte) bool {
	return b >= 'a' && b <= 'z'
}

func getCtagsCommand() string {
	// Check if ctags is available in PATH
	path, err := exec.LookPath("ctags")
	if err == nil {
		return path
	}
	// Fallback or check for universal-ctags / exuberant-ctags?
	// The standard `ctags` command is usually sufficient if configured correctly.
	return "ctags"
}

//export parser
func parser(param *C.struct_parser_param) {
	if param == nil {
		return
	}

	filename := C.GoString(param.file)
	if param.langmap != nil {
		langMapStr := C.GoString(param.langmap)
		parseLangMap(langMapStr)
	}

	// Determine Lexer
	ext := filepath.Ext(filename)
	var lang string
	if langMap != nil {
		lang = langMap[ext]
	}

	var lexer chroma.Lexer
	if lang != "" {
		lexerName := strings.ToLower(lang)
		if alias, exists := languageAliases[lexerName]; exists {
			lexerName = alias
		}
		lexer = lexers.Get(lexerName)
	}
	if lexer == nil {
		lexer = lexers.Match(filename)
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}

	// 1. Get Definitions from Ctags
	// ctags gives definitions (PARSER_DEF)
	defs, err := getCtagsDefinitions(filename)
	if err != nil {
		// Warn?
		// msg := C.CString(fmt.Sprintf("ctags failed: %v", err))
		// C.call_warning(param, msg)
		// C.free(unsafe.Pointer(msg))
	}

	// 2. Get References from Chroma
	// chroma gives references (PARSER_REF_SYM)
	refs, err := getChromaReferences(filename, lexer)
	if err != nil {
		// Warn?
	}

	// 3. Merge
	// We merge such that definitions override references at the same location.
	finalTags := make([]Tag, 0, len(defs)+len(refs))
	defMap := make(map[string]bool) // key: "tag:line"

	for _, d := range defs {
		finalTags = append(finalTags, d)
		key := fmt.Sprintf("%s:%d", d.Tag, d.Line)
		defMap[key] = true
	}

	for _, r := range refs {
		key := fmt.Sprintf("%s:%d", r.Tag, r.Line)
		if !defMap[key] {
			finalTags = append(finalTags, r)
		}
	}

	// Sort tags
	sort.Sort(ByLine(finalTags))

	cFilename := C.CString(filename)
	defer C.free(unsafe.Pointer(cFilename))

	for _, t := range finalTags {
		cTag := C.CString(t.Tag)
		cImage := C.CString(t.Image)

		C.call_put(param, C.int(t.Type), cTag, C.int(t.Line), cFilename, cImage, param.arg)

		C.free(unsafe.Pointer(cTag))
		C.free(unsafe.Pointer(cImage))
	}
}

func getCtagsDefinitions(filename string) ([]Tag, error) {
	// Run ctags -x
	// Note: We need to handle the case where ctags is not found or fails.

	cmd := exec.Command(getCtagsCommand(), "-xu", "--filter", "--filter-terminator=###terminator###\n", "--format=1")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	// Also capture stderr to avoid hanging if buffer fills?
	// cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// Write filename to stdin
	// We must ensure newline at end
	io.WriteString(stdin, filename+"\n")
	stdin.Close()

	reader := bufio.NewReader(stdout)
	var tags []Tag

	// Precompile regex? (Optimization, but doing it here is fine for now)
	pattern := `^(\S+)\s+(\d+)\s+` + regexp.QuoteMeta(filename) + `\s+(.*)$`
	re, err := regexp.Compile(pattern)
	if err != nil {
		// Fallback to simpler regex if QuoteMeta fails for some reason (unlikely)
		return nil, err
	}

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				// return nil, err
			}
			break
		}
		if strings.HasPrefix(line, "###terminator###") {
			break
		}

		matches := re.FindStringSubmatch(strings.TrimSpace(line))
		if len(matches) == 4 {
			tagVal := matches[1]
			lnum, _ := strconv.Atoi(matches[2])
			image := matches[3]

			tags = append(tags, Tag{
				Type:  int(C.PARSER_DEF), // 1
				Tag:   tagVal,
				Line:  lnum,
				File:  filename,
				Image: image,
			})
		}
	}
	cmd.Wait()
	return tags, nil
}

func getChromaReferences(filename string, lexer chroma.Lexer) ([]Tag, error) {
	// Read file content
	contents, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	iterator, err := lexer.Tokenise(nil, string(contents))
	if err != nil {
		return nil, err
	}

	var tags []Tag
	line := 1

	for _, token := range iterator.Tokens() {
		// token.Type is a TokenType (int alias)
		// We want to verify if it is a Name (identifier).
		// chroma.Name is the category.

		// Note: we should handle split tokens?
		// Tokenise returns tokens in order.

		if token.Type == chroma.Name || token.Type.InCategory(chroma.Name) {
			val := strings.TrimSpace(token.Value)
			// Filter out empty or single punctuation chars if desired?
			// Generally identifiers are good.
			// However, some lexers might tag things as Name which are not useful references.
			// But for now, we follow the "Token in Token.Name" logic of the python script.

			if val != "" {
				tags = append(tags, Tag{
					Type:  int(C.PARSER_REF_SYM), // 2
					Tag:   val,
					Line:  line,
					File:  filename,
					Image: "", // No image context for references usually
				})
			}
		}

		line += strings.Count(token.Value, "\n")
	}
	return tags, nil
}

func main() {}
