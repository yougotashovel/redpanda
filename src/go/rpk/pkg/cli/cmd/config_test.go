package cmd_test

import (
	"path/filepath"
	"strings"
	"testing"
	"vectorized/pkg/cli/cmd"
	"vectorized/pkg/config"

	"github.com/spf13/afero"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestSet(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		value     string
		args      []string
		expected  interface{}
		expectErr bool
	}{
		{
			name:     "it should set single integer fields",
			key:      "redpanda.node_id",
			value:    "54312",
			expected: 54312,
		},
		{
			name:     "it should set single float fields",
			key:      "redpanda.float_field",
			value:    "42.3",
			expected: 42.3,
		},
		{
			name:     "it should set single string fields",
			key:      "redpanda.data_directory",
			value:    "'/var/lib/differentdir'",
			expected: "'/var/lib/differentdir'",
		},
		{
			name:     "it should set single bool fields",
			key:      "rpk.enable_usage_stats",
			value:    "true",
			expected: true,
		},
		{
			name:  "it should partially set map fields (yaml)",
			key:   "rpk",
			value: `tune_disk_irq: true`,
			args:  []string{"--format", "yaml"},
			expected: map[string]interface{}{
				"enable_usage_stats": false,
				"tls": map[string]interface{}{
					"cert_file":       "",
					"key_file":        "",
					"truststore_file": "",
				},
				"overprovisioned":            false,
				"tune_network":               false,
				"tune_disk_scheduler":        false,
				"tune_disk_nomerges":         false,
				"tune_disk_irq":              true,
				"tune_cpu":                   false,
				"tune_aio_events":            false,
				"tune_clocksource":           false,
				"tune_swappiness":            false,
				"tune_transparent_hugepages": false,
				"enable_memory_locking":      false,
				"tune_fstrim":                false,
				"tune_coredump":              false,
				"coredump_dir":               "/var/lib/redpanda/coredump",
			},
		},
		{
			name: "it should partially set map fields (json)",
			key:  "redpanda.kafka_api",
			value: `{
  "address": "192.168.54.2"
}`,
			args: []string{"--format", "json"},
			expected: map[string]interface{}{
				"address": "192.168.54.2",
				"port":    9092,
			},
		},
		{
			name:      "it should fail if the new value is invalid",
			key:       "redpanda",
			value:     `{"data_directory": ""}`,
			args:      []string{"--format", "json"},
			expectErr: true,
		},
		{
			name:      "it should fail if the value isn't well formatted (json)",
			key:       "redpanda",
			value:     `{"seed_servers": []`,
			args:      []string{"--format", "json"},
			expectErr: true,
		},
		{
			name: "it should fail if the value isn't well formatted (yaml)",
			key:  "redpanda",
			value: `seed_servers:
- host:
  address: "123.`,
			args:      []string{"--format", "yaml"},
			expectErr: true,
		},
		{
			name:      "it should fail if the format isn't supported",
			key:       "redpanda",
			value:     `node_id=1`,
			args:      []string{"--format", "toml"},
			expectErr: true,
		},
		{
			name:      "it should fail if no key is passed",
			value:     `node_id=1`,
			expectErr: true,
		},
		{
			name:      "it should fail if no value is passed",
			key:       "rpk.tune_coredump",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			conf := config.DefaultConfig()
			err := config.WriteConfig(fs, &conf, conf.ConfigFile)
			require.NoError(t, err)

			c := cmd.NewConfigCommand(fs)
			args := []string{"set"}
			if tt.key != "" {
				args = append(args, tt.key)
			}
			if tt.value != "" {
				args = append(args, tt.value)
			}
			c.SetArgs(append(args, tt.args...))
			err = c.Execute()
			if tt.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			v := viper.New()
			v.SetFs(fs)
			v.SetConfigType("yaml")
			v.SetConfigFile(conf.ConfigFile)
			err = v.ReadInConfig()
			require.NoError(t, err)
			val := v.Get(tt.key)
			require.Exactly(t, tt.expected, val)
		})
	}
}

func TestBootstrap(t *testing.T) {
	tests := []struct {
		name        string
		ips         []string
		self        string
		id          string
		expectedErr string
	}{
		{
			name: "it should set the root node config for a single node",
			id:   "1",
			self: "192.168.34.5",
		},
		{
			name: "it should fill the seed servers",
			ips:  []string{"187.89.76.3", "192.168.34.5", "192.168.45.8"},
			self: "192.168.34.5",
			id:   "1",
		},
		{
			name:        "it should fail if any of the --ips IPs isn't valid",
			ips:         []string{"187.89.9", "192.168.34.5", "192.168.45.8"},
			self:        "192.168.34.5",
			id:          "1",
			expectedErr: "187.89.9 is not a valid IP.",
		},
		{
			name:        "it should fail if --self isn't a valid IP",
			ips:         []string{"187.89.9.78", "192.168.34.5", "192.168.45.8"},
			self:        "www.host.com",
			id:          "1",
			expectedErr: "www.host.com is not a valid IP.",
		},
		{
			name:        "it should fail if --id isn't passed",
			expectedErr: "required flag(s) \"id\" not set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := "./redpanda.yaml"
			fs := afero.NewMemMapFs()
			err := fs.MkdirAll(
				filepath.Dir(configPath),
				0644,
			)
			require.NoError(t, err)
			c := cmd.NewConfigCommand(fs)
			args := []string{"bootstrap", "--config", configPath}
			if len(tt.ips) != 0 {
				args = append(
					args,
					"--ips",
					strings.Join(tt.ips, ","),
				)
			}
			if tt.self != "" {
				args = append(args, "--self", tt.self)
			}
			if tt.id != "" {
				args = append(args, "--id", tt.id)
			}
			c.SetArgs(args)
			err = c.Execute()
			if tt.expectedErr != "" {
				require.EqualError(t, err, tt.expectedErr)
				return
			}
			require.NoError(t, err)
			_, err = fs.Stat(configPath)
			conf, err := config.ReadConfigFromPath(fs, configPath)
			require.NoError(t, err)
			require.Equal(t, conf.Redpanda.RPCServer.Address, tt.self)
			require.Equal(t, conf.Redpanda.KafkaApi.Address, tt.self)
			require.Equal(t, conf.Redpanda.AdminApi.Address, tt.self)
			if len(tt.ips) == 1 {
				require.Equal(
					t,
					[]*config.SeedServer{},
					conf.Redpanda.SeedServers,
				)
				return
			}
			seedAddrs := []string{}
			for _, seed := range conf.Redpanda.SeedServers {
				seedAddrs = append(seedAddrs, seed.Host.Address)
			}
			require.ElementsMatch(t, tt.ips, seedAddrs)
		})
	}
}

func TestInitNode(t *testing.T) {
	fs := afero.NewMemMapFs()
	conf := config.DefaultConfig()
	err := config.WriteConfig(fs, &conf, conf.ConfigFile)
	require.NoError(t, err)
	c := cmd.NewConfigCommand(fs)
	args := []string{"init"}
	c.SetArgs(args)

	err = c.Execute()
	require.NoError(t, err)

	v := viper.New()
	v.SetFs(fs)
	v.SetConfigType("yaml")
	v.SetConfigFile(conf.ConfigFile)
	err = v.ReadInConfig()
	require.NoError(t, err)

	val := v.Get("node_uuid")
	require.NotEmpty(t, val)
}
