package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const timeLayout = "2006-01-02 15:04:05-07:00"
const timeDelimiter = "=>"

var never = time.Time{}

//e,g, 2017-01-10 17:31:04+01:00 - 2017-01-10 17:31:08+01:00.txt
type log string

func loadLog(path string) (log, error) {
	src, err := os.Stat(path)
	l := log(path)
	if err != nil || src.IsDir() || l.start() == never || l.end() == never {
		return log(""), errors.New("Invalid Log File: " + path)
	}
	return l, nil
}

func (l log) path() string {
	return string(l)
}

func (l log) dir() string {
	dir, _ := filepath.Split(l.path())
	if dir == "" {
		return "./"
	}
	return dir
}

func (l log) name() string {
	_, name := filepath.Split(l.path())
	return strings.TrimSuffix(name, ".txt")
}

func (l log) text() string {
	b, err := ioutil.ReadFile(l.path())
	if err != nil {
		panic(err)
	}
	return string(b) //l.start().String() + " " + l.duration().String() + "\t\t" + l.dir() + "\n" + string(b)
}

func (l log) start() time.Time {
	nameSplit := strings.SplitN(l.name(), timeDelimiter, 2)

	start, err := time.Parse(timeLayout, nameSplit[0])
	if err != nil {
		return never
	}

	return start
}

func (l log) end() time.Time {
	nameSplit := strings.SplitN(l.name(), timeDelimiter, 2)

	end, err := time.Parse(timeLayout, nameSplit[1])
	if err != nil {
		return never
	}

	return end
}

func (l log) duration() time.Duration {
	return l.end().Sub(l.start())
}

type task string

func createTask(path string) error {
	return os.MkdirAll(path, 0700)
}

func loadTask(path string) (task, error) {
	src, err := os.Stat(path)

	if err != nil || !src.IsDir() {
		return task(""), errors.New("Invalid Task Directory: " + path)
	}
	return task(path), nil
}

func (t task) path() string {
	return string(t)
}

func (t task) recursiveDurationWithin(dur time.Duration) time.Duration {
	var total time.Duration
	for _, l := range t.logsWithin(dur) {
		total += l.duration()
	}
	for _, t := range t.subtasks() {
		total += t.recursiveDurationWithin(dur)
	}
	return total
}

func (t task) durationWithin(dur time.Duration) time.Duration {
	var total time.Duration
	for _, l := range t.logsWithin(dur) {
		total += l.duration()
	}
	return total
}

func (t task) summaryWithin(dur time.Duration) string {
	var answer string
	ls := t.logsWithin(dur)
	if len(ls) > 0 {
		answer += t.path() + " (" + t.durationWithin(dur).String() + ")\n"
	}

	ts := t.subtasks()
	for _, t2 := range ts {
		answer += t2.summaryWithin(dur)
	}
	return answer
}

func (t task) textWithin(dur time.Duration) string {
	var answer string
	ls := t.logsWithin(dur)
	if len(ls) > 0 {
		title := t.path() + " (" + t.durationWithin(dur).String() + ")\n"
		answer += title
		for _, l := range ls {
			answer += l.text()
		}
		answer += "\n"
	}

	ts := t.subtasks()
	for _, t2 := range ts {
		answer += t2.textWithin(dur)
	}
	return answer
}

type logs []log

type logsByStart logs

func (lbs logsByStart) Len() int {
	return len(lbs)
}

func (lbs logsByStart) Less(i, j int) bool {
	//less reports whether i should sort before j
	return lbs[i].start().Before(lbs[j].start())
}

func (lbs logsByStart) Swap(i, j int) {
	temp := lbs[i]
	lbs[i] = lbs[j]
	lbs[j] = temp
}

type logsByEnd logs

func (lbe logsByEnd) Len() int {
	return len(lbe)
}

func (lbe logsByEnd) Less(i, j int) bool {
	return lbe[i].end().Before(lbe[j].end())
}

func (lbe logsByEnd) Swap(i, j int) {
	temp := lbe[i]
	lbe[i] = lbe[j]
	lbe[j] = temp
}

func (t task) logsWithin(dur time.Duration) logs {
	var answer logs
	files, _ := ioutil.ReadDir(t.path())
	for _, f := range files {
		l, err := loadLog(t.path() + "/" + f.Name())
		if err != nil {
			continue
		}
		if dur == 0 || l.end().After(time.Now().Add(-dur)) {
			answer = append(answer, l)
		}
	}
	return answer
}

func (t task) recursiveLogsWithin(dur time.Duration) logs {
	var answer logs
	answer = append(answer, t.logsWithin(dur)...)
	for _, t2 := range t.subtasks() {
		answer = append(answer, t2.recursiveLogsWithin(dur)...)
	}
	return answer
}

func (t task) subtasks() []task {
	var answer []task
	files, _ := ioutil.ReadDir(t.path())
	for _, f := range files {
		t2, err := loadTask(t.path() + "/" + f.Name())
		if err != nil {
			continue
		}
		answer = append(answer, t2)
	}
	return answer
}

//
// func getScreenLockState() bool {
// 	cmd := exec.Command("qdbus", "org.gnome.ScreenSaver", "/org/gnome/ScreenSaver", "org.gnome.ScreenSaver.GetActive")
// 	var outb bytes.Buffer
// 	cmd.Stdout = &outb
// 	err := cmd.Run()
// 	if err != nil {
// 		panic(err)
// 	}
// 	if outb.String() == "true\n" {
// 		return true
// 	}
// 	return false
// }

func parseDurationArgument(arg string) time.Duration {
	var dur time.Duration
	args := strings.SplitN(arg, "=", 2)
	if len(args) == 1 {
		return 0
	}
	arg = args[1]
	if len(arg) == 0 {
		panic(errors.New("No duration specified"))
	}
	if arg[len(arg)-1:len(arg)] == "d" {
		days, err := strconv.Atoi(arg[:len(arg)-1])
		if err != nil {
			panic(err)
		}
		dur = time.Hour * 24 * time.Duration(days)
	} else {
		var err error
		dur, err = time.ParseDuration(arg)
		if err != nil {
			panic(err)
		}
	}
	return dur
}

func (t task) createLog() error {
	fpath := os.TempDir() + "/" + strings.Replace(t.path(), "/", "⧸", -1) + ".log"
	f, err := os.Create(fpath)
	if err != nil {
		return err
	}
	f.Close()

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	editCmd := exec.Command(editor, fpath)
	editCmd.Stdin = os.Stdin
	editCmd.Stdout = os.Stdout
	editCmd.Stderr = os.Stderr
	startT := time.Now()
	defer func() {
		endT := time.Now()
		dpath := t.path() + "/" + startT.Format(timeLayout) + timeDelimiter + endT.Format(timeLayout) + ".txt"
		cpCmd := exec.Command("cp", fpath, dpath)
		err = cpCmd.Run()
		if err != nil {
			touchCmd := exec.Command("touch", dpath)
			touchCmd.Run()
		}
	}()
	err = editCmd.Start()
	if err != nil {
		return err
	}
	err = editCmd.Wait()
	if err != nil {
		return err
	}

	return err
}

func main() {
	args := os.Args[1:]
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		//print help
		fmt.Println(`horolog v1.4

Usage:
	horolog task123/investigation
 		Starts logging in specified task

Options:
	-s/--show
		Displays total time, time of each subtask, and all logged text
	-s=/--show=
		The same as --show, but also filters out activity older than the
		specified length of time (units are d/h/m/s)
	-u/--summary
		Only show total time and time of each subtask
	-u=/--summary=
		The same as --summary, but also filters out activity older than
		the specified length of time (units are d/h/m/s)
	-t/--timeline
		Displays time spent on tasks, in order
	-t=/--timeline=
		The same as --timeline, but also filters out activity older than
		the specified lenght of time (units are d/h/m/s)
	-a=/--ammend=
		Retroactively adds the specified time to a task (can be negative)
	-h/--help
		Displays this text`)
	} else if len(args) > 0 && (strings.HasPrefix(args[0], "--timeline") || strings.HasPrefix(args[0], "-t")) {
		dur := parseDurationArgument(args[0])
		var dir string
		if len(args) == 1 {
			dir = "."
		} else {
			dir = args[1]
		}
		t, err := loadTask(dir)
		if err != nil {
			panic(err)
		}
		ls := t.recursiveLogsWithin(dur)
		sort.Sort(logsByEnd(ls))
		for _, l := range ls {
			fmt.Println(l.start(), l.duration(), "\t\t", l.dir())
			fmt.Println(l.text())
		}
	} else if len(args) > 0 && (strings.HasPrefix(args[0], "--ammend") || strings.HasPrefix(args[0], "-a")) {
		dur := parseDurationArgument(args[0])
		var dir string
		if len(args) == 1 {
			dir = "."
		} else {
			dir = args[1]
		}
		t, err := loadTask(dir)
		if err != nil {
			err = createTask(dir)
			if err != nil {
				panic(err)
			}
			t, err = loadTask(dir)
			if err != nil {
				panic(err)
			}
		}
		startT := time.Now().Add(-dur)
		endT := time.Now()
		p := t.path() + "/" + startT.Format(timeLayout) + timeDelimiter + endT.Format(timeLayout) + ".txt"
		touchCmd := exec.Command("touch", p)
		touchCmd.Run()
	} else if len(args) > 0 && (strings.HasPrefix(args[0], "--show") || strings.HasPrefix(args[0], "-s")) {
		var dir string
		if len(args) == 1 {
			dir = "."
		} else {
			dir = args[1]
		}
		dur := parseDurationArgument(args[0])

		//handle --show=dur
		t, err := loadTask(dir)
		if err != nil {
			panic(err)
		}

		fmt.Println("Total: " + t.recursiveDurationWithin(dur).String() + "\n")
		fmt.Println(t.textWithin(dur))

	} else if len(args) > 0 && (strings.HasPrefix(args[0], "--summary") || strings.HasPrefix(args[0], "-u")) {
		var dir string
		if len(args) == 1 {
			dir = "."
		} else {
			dir = args[1]
		}
		dur := parseDurationArgument(args[0])

		//handle --show=dur
		t, err := loadTask(dir)
		if err != nil {
			panic(err)
		}

		fmt.Println("Total: " + t.recursiveDurationWithin(dur).String() + "\n")
		fmt.Println(t.summaryWithin(dur))

	} else {
		var dir string

		if len(args) == 0 {
			dir = "."
		} else {
			dir = args[0]
		}

		//handle creation
		t, err := loadTask(dir)
		if err != nil {
			err = createTask(dir)
			if err != nil {
				panic(err)
			}
			t, err = loadTask(dir)
			if err != nil {
				panic(err)
			}
		}
		t.createLog()
	}
}
