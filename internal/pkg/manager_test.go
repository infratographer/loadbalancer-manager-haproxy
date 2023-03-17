package pkg

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	parser "github.com/haproxytech/config-parser/v4"
	"github.com/haproxytech/config-parser/v4/options"
	"github.com/stretchr/testify/assert"
	"go.infratographer.com/loadbalancer-manager-haproxy/internal/lbapi"
)

const (
	testDataBaseDir = "testdata"
)

func TestMergeConfig(t *testing.T) {
	t.Run("fails with no assignments", func(t *testing.T) {
		t.Parallel()

		lbRespJson := `{
  "id": "58622a8d-54a2-4b0c-8b5f-8de7dff29f6f",
  "assignments": []
}
`
		lb := lbapi.LoadBalancer{}
		err := json.NewDecoder(strings.NewReader(lbRespJson)).Decode(&lb)
		assert.Nil(t, err)
		t.Logf("%+v", lb)

		cfg, err := parser.New(options.Path("../../.devcontainer/config/haproxy.cfg"), options.NoNamedDefaultsFrom)
		assert.Nil(t, err)

		newCfg, err := mergeConfig(cfg, &lb)
		assert.Nil(t, newCfg)
		assert.NotNil(t, err)
	})

	MergeConfigTests := []struct {
		name                string
		jsonInputFilename   string
		expectedCfgFilename string
	}{
		{"ssh service one pool", "lb-resp-ex-1.json", "lb-ex-1-exp.cfg"},
		{"ssh service two pools", "lb-resp-ex-2.json", "lb-ex-2-exp.cfg"},
		{"http and https", "lb-resp-ex-3.json", "lb-ex-3-exp.cfg"},
	}

	for _, tt := range MergeConfigTests {
		// go vet
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			jsonResp, err := os.Open(fmt.Sprintf("%s/%s", testDataBaseDir, tt.jsonInputFilename))
			assert.Nil(t, err)
			defer jsonResp.Close()

			lb := lbapi.LoadBalancer{}
			err = json.NewDecoder(jsonResp).Decode(&lb)
			assert.Nil(t, err)
			t.Logf("%+v", lb)

			cfg, err := parser.New(options.Path("../../.devcontainer/config/haproxy.cfg"), options.NoNamedDefaultsFrom)
			assert.Nil(t, err)

			newCfg, err := mergeConfig(cfg, &lb)
			assert.Nil(t, err)

			t.Log("Generated config ===> ", newCfg.String())

			expCfg, err := os.ReadFile(fmt.Sprintf("%s/%s", testDataBaseDir, tt.expectedCfgFilename))
			assert.Nil(t, err)

			assert.Equal(t, strings.TrimSpace(string(expCfg)), strings.TrimSpace(newCfg.String()))
		})
	}
}
