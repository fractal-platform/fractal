package log

import (
	"bytes"
	"strings"
	"testing"
)

func TestLog15Logger_Info(t *testing.T) {
	var buffer []byte
	var writer = bytes.NewBuffer(buffer)
	SetDefaultLogger(InitLog15Logger(LvlInfo, writer))
	Info("test")
	output := writer.String()
	if strings.Contains(output, "msg=test") {
		t.Logf("Normal log test OK")
	} else {
		t.Errorf("Normal log test Failed")
	}
}

func TestLog15Logger_Lazy(t *testing.T) {
	var buffer []byte
	var writer = bytes.NewBuffer(buffer)
	SetDefaultLogger(InitLog15Logger(LvlInfo, writer))
	Debug("test", "param", Lazy{
		Fn: func() string {
			// should not exec this func
			t.Errorf("Lazy log test Failed")
			return "non-exist"
		},
	})
	//output := string(writer.Bytes())
	//t.Logf(output)
	t.Logf("Normal lazy test OK")
}
