package cniovs

//
// Fake implementation of execCommand suitable for unit testing
//

type FakeExecCommand struct {
	Out  []byte
	Err  error
	Cmd  string
	Args []string
}

func (e *FakeExecCommand) execCommand(cmd string, args []string) ([]byte, error) {
	e.Cmd = cmd
	e.Args = args
	return e.Out, e.Err
}
