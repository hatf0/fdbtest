package fdbtest

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"testing"

	"github.com/apple/foundationdb/bindings/go/src/fdb"
	"github.com/pkg/errors"
)

var verboseFdb = flag.Bool("verbose-fdb", false, "make foundationdb verbose")

type fdbServer struct {
	version     string
	dockerID    string
	clusterFile string
	DB          fdb.Database
}

func NewServer(version string) *fdbServer {
	s := &fdbServer{
		version: version,
	}
	return s
}

func (s *fdbServer) ClusterFile() string {
	return s.clusterFile
}

func (s *fdbServer) MustClear(t testing.TB) {
	t.Helper()

	err := s.Clear(t)
	if err != nil {
		panic(err)
	}
}

func (s *fdbServer) Clear(t testing.TB) error {
	t.Helper()

	_, err := s.DB.Transact(func(tx fdb.Transaction) (interface{}, error) {
		tx.ClearRange(fdb.KeyRange{
			Begin: fdb.Key([]byte{0x00}),
			End:   fdb.Key([]byte{0xff}),
		})
		return nil, nil
	})

	return err
}

// Destroy destroys the foundationdb cluster.
func (s *fdbServer) Destroy(t testing.TB) error {
	t.Helper()
	return exec.Command("docker", "stop", s.dockerID).Run()
}

var DefaultServer *fdbServer

func MustStart(t testing.TB) *fdbServer {
	t.Helper()

	if DefaultServer == nil {
		DefaultServer = NewServer("7.3.43")
	}

	DefaultServer.MustStart(t)
	return DefaultServer
}

// MustStart starts a new foundationdb node.
func (s *fdbServer) MustStart(t testing.TB) {
	t.Helper()

	if err := s.Start(t); err != nil {
		t.Fatalf("FdbServer.MustStart(): start failed with %v", err)
	}
}

// Start starts a new foundationdb cluster.
func Start(t testing.TB) error {
	t.Helper()

	if DefaultServer == nil {
		DefaultServer = NewServer("7.3.43")
	}

	return DefaultServer.Start(t)
}

func (s *fdbServer) Start(t testing.TB) error {
	t.Helper()

	if s.version == "" {
		s.version = "7.3.43"
	}

	// start new foundationdb docker container
	runCmd := exec.Command("docker", "run", "--rm", "--detach", fmt.Sprintf("foundationdb/foundationdb:%s", s.version))
	if *verboseFdb {
		t.Logf("+%v\n", runCmd.String())
	}

	output, err := runCmd.Output()
	if *verboseFdb {
		t.Logf(string(output))
	}
	if err != nil {
		return errors.Wrap(err, "docker run failed")
	}

	// get docker id from output
	dockerID := strings.TrimSpace(string(output))
	if len(dockerID) != 64 {
		return errors.New("invalid docker id in stdout: " + dockerID)
	}
	// trim docker id
	s.dockerID = dockerID[:12]

	t.Logf("foundationdb container started: %v\n", dockerID)
	t.Cleanup(func() {
		s.Destroy(t)
	})

	// initialize new database
	initCmd := exec.Command("docker", "exec", dockerID, "fdbcli", "--exec", "configure new single ssd")
	if *verboseFdb {
		t.Logf("+%v\n", initCmd.String())
	}

	output, err = initCmd.CombinedOutput()
	if err != nil {
		t.Logf("initialize database error: %v\r\n\r\n%v\n", err, string(output))
		return errors.Wrap(err, "docker exec failed: "+string(output))
	}

	if !strings.Contains(string(output), "Database created") {
		return errors.New("unexpected configure database output: " + string(output))
	}

	if *verboseFdb {
		t.Logf("database initialize command succeeded: %v\n", strings.TrimSpace(string(output)))
	}
	// get container ip
	inspectCmd := exec.Command("docker", "inspect", dockerID, "-f", "{{ .NetworkSettings.Networks.bridge.IPAddress }}")
	if *verboseFdb {
		t.Logf("+%v\n", inspectCmd.String())
	}
	output, err = inspectCmd.CombinedOutput()
	if err != nil {
		t.Logf("container network ip lookup failed: %v\r\n\r\n%v", err, string(output))
		return errors.Wrap(err, "docker exec inspect: "+string(output))
	}
	ipAddress := strings.TrimSpace(string(output))

	// validate ip
	matched, err := regexp.MatchString("^[0-9]{1,3}\\.[0-9]{1,3}\\.[0-9]{1,3}\\.[0-9]{1,3}$", ipAddress)
	if err != nil {
		return errors.Wrap(err, "ip address regex match error")
	}

	if !matched {
		return errors.New("invalid ip address: " + ipAddress)
	}

	// generate unique cluster file
	clusterFile, err := os.Create(path.Join(t.TempDir(), "fdb.cluster"))
	if err != nil {
		return err
	}
	defer clusterFile.Close()
	s.clusterFile = clusterFile.Name()
	cluster := fmt.Sprintf("docker:docker@%v:4500", string(ipAddress))
	if _, err := clusterFile.Write([]byte(cluster)); err != nil {
		return fmt.Errorf("writing cluster file contents: %w", err)
	}

	t.Logf("cluster available: %v\n", cluster)

	version, err := fdb.GetAPIVersion()
	if err != nil {
		return fmt.Errorf("error getting API version: %w", err)
	}

	t.Logf("foundationdb client api version: %v\n", version)

	db, err := fdb.OpenDatabase(s.clusterFile)
	if err != nil {
		return fmt.Errorf("error opening database: %w", err)
	}
	s.DB = db

	return nil
}
