package keys

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"
)

func TestKeyJson(t *testing.T) {
	bytes, err := ioutil.ReadFile("./key.json")
	if err != nil {
		fmt.Println(err)
		return
	}

	keyJSON := new(keyfileJSON)
	err = json.Unmarshal(bytes, &keyJSON)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(keyJSON)
}
