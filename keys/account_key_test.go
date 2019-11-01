package keys

import (
	"os"
	"testing"

	"github.com/fractal-platform/fractal/utils/log"
)

func TestAccountKeySign(t *testing.T) {
	log.SetDefaultLogger(log.InitLog15Logger(log.LvlDebug, os.Stdout))
	key, err := LoadAccountKey("./key.json", "12345")
	if err != nil {
		return
	}
	key.PrivKey.Sign([]byte{})
}
