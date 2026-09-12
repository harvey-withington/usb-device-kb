// submit turns a submission issue into a change to docks.json.
//
//	go run ./cmd/submit issue-body.md
//
// The issue form puts the entry in a ```json fenced block; the tool takes
// the first such block (or the whole body when it is bare JSON), fills in
// an id from the name when there is none, drops generic hubs, validates
// the entry on its own and against the file, merges it, and writes the
// file. It prints a summary for the pull request and, under GitHub
// Actions, sets the outputs `summary` and `dock_id`. A problem exits 1
// with the reasons listed, which the workflow posts back on the issue.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/harvey-withington/usb-device-kb/kbdata"
)

var fenced = regexp.MustCompile("(?s)```(?:json)?\\s*\\n(.*?)\\n\\s*```")

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: submit issue-body.md")
		os.Exit(2)
	}
	body, err := os.ReadFile(os.Args[1])
	if err != nil {
		fail(err.Error())
	}
	entry, err := parseEntry(string(body))
	if err != nil {
		fail("could not read the entry: " + err.Error())
	}

	file, err := kbdata.LoadDocks("docks.json")
	if err != nil {
		fail(err.Error())
	}
	kbdata.StripGenericHubs(&entry, file.GenericInternalHubs)
	if entry.ID == "" || strings.HasPrefix(entry.ID, "local:") {
		entry.ID = kbdata.Slug(entry.Name)
	}
	if entry.Verified == "" {
		entry.Verified = "submitted through the issue form"
	}
	if problems := kbdata.ValidateDock(entry, file.GenericInternalHubs); len(problems) > 0 {
		fail("the entry has problems:\n- " + strings.Join(problems, "\n- "))
	}
	replaced, err := kbdata.MergeDock(file, entry)
	if errors.Is(err, kbdata.ErrConflict) {
		fail("the entry overlaps another dock, which a maintainer has to settle by hand: " + err.Error())
	} else if err != nil {
		fail(err.Error())
	}
	if problems := kbdata.ValidateDocks(file); len(problems) > 0 {
		fail("the file would have problems after merging:\n- " + strings.Join(problems, "\n- "))
	}
	if err := kbdata.WriteDocks("docks.json", file); err != nil {
		fail(err.Error())
	}

	summary := fmt.Sprintf("Add %s (`%s`, %d hub(s))", entry.Name, entry.ID, len(entry.Hubs))
	if replaced != "" {
		summary = fmt.Sprintf("Update %s (`%s`, %d hub(s))", entry.Name, replaced, len(entry.Hubs))
	}
	fmt.Println(summary)
	setOutput("summary", summary)
	setOutput("dock_id", entry.ID)
}

// parseEntry finds the JSON entry in an issue body.
func parseEntry(body string) (kbdata.Dock, error) {
	raw := strings.TrimSpace(body)
	if m := fenced.FindStringSubmatch(body); m != nil {
		raw = strings.TrimSpace(m[1])
	}
	var entry kbdata.Dock
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&entry); err != nil {
		return entry, err
	}
	return entry, nil
}

func setOutput(name, value string) {
	path := os.Getenv("GITHUB_OUTPUT")
	if path == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s=%s\n", name, value)
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
