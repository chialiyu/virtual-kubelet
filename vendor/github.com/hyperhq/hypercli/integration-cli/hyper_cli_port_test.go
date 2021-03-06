package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestCliPortList(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	// one port
	out, _ := dockerCmd(c, "run", "-d", "-p", "9876:80", "busybox", "top")
	firstID := strings.TrimSpace(out)

	out, _ = dockerCmd(c, "port", firstID, "80")

	err := assertPortList(c, out, []string{"0.0.0.0:9876"})
	// Port list is not correct
	c.Assert(err, checker.IsNil)

	out, _ = dockerCmd(c, "port", firstID)

	err = assertPortList(c, out, []string{"80/tcp -> 0.0.0.0:9876"})
	// Port list is not correct
	c.Assert(err, checker.IsNil)

	dockerCmd(c, "rm", "-f", firstID)

	// three port
	out, _ = dockerCmd(c, "run", "-d",
		"-p", "9876:80",
		"-p", "9877:81",
		"-p", "9878:82",
		"busybox", "top")
	ID := strings.TrimSpace(out)

	out, _ = dockerCmd(c, "port", ID, "80")

	err = assertPortList(c, out, []string{"0.0.0.0:9876"})
	// Port list is not correct
	c.Assert(err, checker.IsNil)

	out, _ = dockerCmd(c, "port", ID)

	err = assertPortList(c, out, []string{
		"80/tcp -> 0.0.0.0:9876",
		"81/tcp -> 0.0.0.0:9877",
		"82/tcp -> 0.0.0.0:9878"})
	// Port list is not correct
	c.Assert(err, checker.IsNil)

	dockerCmd(c, "rm", "-f", ID)

	// more and one port mapped to the same container port
	out, _ = dockerCmd(c, "run", "-d",
		"-p", "9876:80",
		"-p", "9999:80",
		"-p", "9877:81",
		"-p", "9878:82",
		"busybox", "top")
	ID = strings.TrimSpace(out)

	out, _ = dockerCmd(c, "port", ID, "80")

	err = assertPortList(c, out, []string{"0.0.0.0:9876", "0.0.0.0:9999"})
	// Port list is not correct
	c.Assert(err, checker.IsNil)

	out, _ = dockerCmd(c, "port", ID)

	err = assertPortList(c, out, []string{
		"80/tcp -> 0.0.0.0:9876",
		"80/tcp -> 0.0.0.0:9999",
		"81/tcp -> 0.0.0.0:9877",
		"82/tcp -> 0.0.0.0:9878"})
	// Port list is not correct
	c.Assert(err, checker.IsNil)
	dockerCmd(c, "rm", "-f", ID)

	testRange := func() {
		// host port ranges used
		IDs := make([]string, 3)
		for i := 0; i < 3; i++ {
			out, _ = dockerCmd(c, "run", "-d",
				"-p", "9090-9092:80",
				"busybox", "top")
			IDs[i] = strings.TrimSpace(out)

			out, _ = dockerCmd(c, "port", IDs[i])

			err = assertPortList(c, out, []string{fmt.Sprintf("80/tcp -> 0.0.0.0:%d", 9090)})
			// Port list is not correct
			c.Assert(err, checker.IsNil)
		}

		for i := 0; i < 3; i++ {
			dockerCmd(c, "rm", "-f", IDs[i])
		}
	}
	testRange()
	// Verify we ran re-use port ranges after they are no longer in use.
	testRange()

	// test invalid port ranges
	for _, invalidRange := range []string{"9090-9089:80", "9090-:80", "-9090:80"} {
		out, _, err = dockerCmdWithError("run", "-d",
			"-p", invalidRange,
			"busybox", "top")
		// Port range should have returned an error
		c.Assert(err, checker.NotNil, check.Commentf("out: %s", out))
	}

	// test host range:container range spec.
	out, _ = dockerCmd(c, "run", "-d",
		"-p", "9800-9803:80-83",
		"busybox", "top")
	ID = strings.TrimSpace(out)

	out, _ = dockerCmd(c, "port", ID)

	err = assertPortList(c, out, []string{
		"80/tcp -> 0.0.0.0:9800",
		"81/tcp -> 0.0.0.0:9801",
		"82/tcp -> 0.0.0.0:9802",
		"83/tcp -> 0.0.0.0:9803"})
	// Port list is not correct
	c.Assert(err, checker.IsNil)
	dockerCmd(c, "rm", "-f", ID)

	// test mixing protocols in same port range
	out, _ = dockerCmd(c, "run", "-d",
		"-p", "8000-8080:80",
		"-p", "8000-8080:80/udp",
		"busybox", "top")
	ID = strings.TrimSpace(out)

	out, _ = dockerCmd(c, "port", ID)

	err = assertPortList(c, out, []string{
		"80/tcp -> 0.0.0.0:8000",
		"80/udp -> 0.0.0.0:8000"})
	// Port list is not correct
	c.Assert(err, checker.IsNil)
	dockerCmd(c, "rm", "-f", ID)
}

func assertPortList(c *check.C, out string, expected []string) error {
	lines := strings.Split(strings.Trim(out, "\n "), "\n")
	if len(lines) != len(expected) {
		return fmt.Errorf("different size lists %s, %d, %d", out, len(lines), len(expected))
	}
	sort.Strings(lines)
	sort.Strings(expected)

	for i := 0; i < len(expected); i++ {
		if lines[i] != expected[i] {
			return fmt.Errorf("|" + lines[i] + "!=" + expected[i] + "|")
		}
	}

	return nil
}

func stopRemoveContainer(id string, c *check.C) {
	dockerCmd(c, "rm", "-f", id)
}

func (s *DockerSuite) TestCliPortUnpublishedPortsInPsOutput(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	// Run busybox with command line expose (equivalent to EXPOSE in image's Dockerfile) for the following ports
	port1 := 80
	port2 := 443
	expose1 := fmt.Sprintf("--expose=%d", port1)
	expose2 := fmt.Sprintf("--expose=%d", port2)
	dockerCmd(c, "run", "-d", expose1, expose2, "busybox", "sleep", "5")

	unpPort1 := fmt.Sprintf("%d/tcp", port1)
	unpPort2 := fmt.Sprintf("%d/tcp", port2)

	// Run the container forcing to publish the exposed ports
	dockerCmd(c, "run", "-d", "-P", expose1, expose2, "busybox", "sleep", "5")

	// Check docker ps o/p for last created container reports the exposed ports in the port bindings
	expBndRegx1 := regexp.MustCompile(`0.0.0.0:\d+->` + unpPort1)
	expBndRegx2 := regexp.MustCompile(`0.0.0.0:\d+->` + unpPort2)
	out, _ := dockerCmd(c, "ps", "-n=1")
	// Cannot find expected port binding port (0.0.0.0:xxxxx->unpPort1) in docker ps output
	c.Assert(expBndRegx1.MatchString(out), checker.Equals, true, check.Commentf("out: %s; unpPort1: %s", out, unpPort1))
	// Cannot find expected port binding port (0.0.0.0:xxxxx->unpPort2) in docker ps output
	c.Assert(expBndRegx2.MatchString(out), checker.Equals, true, check.Commentf("out: %s; unpPort2: %s", out, unpPort2))

	// Run the container specifying explicit port bindings for the exposed ports
	offset := 10000
	pFlag1 := fmt.Sprintf("%d:%d", offset+port1, port1)
	pFlag2 := fmt.Sprintf("%d:%d", offset+port2, port2)
	out, _ = dockerCmd(c, "run", "-d", "-p", pFlag1, "-p", pFlag2, expose1, expose2, "busybox", "sleep", "5")
	id := strings.TrimSpace(out)

	// Check docker ps o/p for last created container reports the specified port mappings
	expBnd1 := fmt.Sprintf("0.0.0.0:%d->%s", offset+port1, unpPort1)
	expBnd2 := fmt.Sprintf("0.0.0.0:%d->%s", offset+port2, unpPort2)
	out, _ = dockerCmd(c, "ps", "-n=1")
	// Cannot find expected port binding (expBnd1) in docker ps output
	c.Assert(out, checker.Contains, expBnd1)
	// Cannot find expected port binding (expBnd2) in docker ps output
	c.Assert(out, checker.Contains, expBnd2)

	// Remove container now otherwise it will interfere with next test
	stopRemoveContainer(id, c)

	// Run the container with explicit port bindings and no exposed ports
	out, _ = dockerCmd(c, "run", "-d", "-p", pFlag1, "-p", pFlag2, "busybox", "sleep", "5")
	id = strings.TrimSpace(out)

	// Check docker ps o/p for last created container reports the specified port mappings
	out, _ = dockerCmd(c, "ps", "-n=1")
	// Cannot find expected port binding (expBnd1) in docker ps output
	c.Assert(out, checker.Contains, expBnd1)
	// Cannot find expected port binding (expBnd2) in docker ps output
	c.Assert(out, checker.Contains, expBnd2)
	// Remove container now otherwise it will interfere with next test
	stopRemoveContainer(id, c)

	// Run the container with one unpublished exposed port and one explicit port binding
	dockerCmd(c, "run", "-d", expose1, "-p", pFlag2, "busybox", "sleep", "5")

	// Check docker ps o/p for last created container reports the specified unpublished port and port mapping
	out, _ = dockerCmd(c, "ps", "-n=1")
	// Missing port binding (expBnd2) in docker ps output
	c.Assert(out, checker.Contains, expBnd2)
}

func (s *DockerSuite) TestCliPortHostBindingBasic(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux, NotUserNamespace)

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "-d", "-p", "9876:80", "busybox",
		"nc", "-l", "-p", "80")
	firstID := strings.TrimSpace(out)

	out, _ = dockerCmd(c, "port", firstID, "80")

	err := assertPortList(c, out, []string{"0.0.0.0:9876"})
	// Port list is not correct
	c.Assert(err, checker.IsNil)
}
