package keys

import (
	"github.com/fractal-platform/fractal/utils/log"
	"os"
	"testing"
)

func TestMiningKeySign(t *testing.T) {
	log.SetDefaultLogger(log.InitLog15Logger(log.LvlDebug, os.Stdout))
	manager := NewMiningKeyManager("./", "12345")
	manager.Start()
	//manager.Sign
	manager.Stop()
}
