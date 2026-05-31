package devflow

// MockDevBackup is a no-op BackupRunner for use in tests.
type MockDevBackup struct {
	RunCalled     int
	RunResult     string
	RunErr        error
	CommandStored string
}

func (m *MockDevBackup) SetLog(_ func(...any))       {}
func (m *MockDevBackup) SetCommand(cmd string) error { m.CommandStored = cmd; return nil }
func (m *MockDevBackup) GetCommand() (string, error) { return m.CommandStored, nil }
func (m *MockDevBackup) Run() (string, error) {
	m.RunCalled++
	return m.RunResult, m.RunErr
}
