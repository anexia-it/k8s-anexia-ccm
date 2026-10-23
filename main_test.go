package main

import (
	"reflect"
	"testing"
)

func TestArgumentsWithLoggingEnvironment(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		environment map[string]string
		want        []string
	}{
		{
			name: "adds logging flags from environment",
			args: []string{"--cloud-provider=anexia"},
			environment: map[string]string{
				logFormatEnvironmentVariable: "json",
				logLevelEnvironmentVariable:  "2",
			},
			want: []string{"--logging-format=json", "--v=2", "--cloud-provider=anexia"},
		},
		{
			name: "explicit long flags take precedence",
			args: []string{"--logging-format=text", "--v=4"},
			environment: map[string]string{
				logFormatEnvironmentVariable: "json",
				logLevelEnvironmentVariable:  "2",
			},
			want: []string{"--logging-format=text", "--v=4"},
		},
		{
			name: "explicit short verbosity flag takes precedence",
			args: []string{"-v", "5"},
			environment: map[string]string{
				logLevelEnvironmentVariable: "2",
			},
			want: []string{"-v", "5"},
		},
		{
			name: "ignores empty environment values",
			environment: map[string]string{
				logFormatEnvironmentVariable: " ",
				logLevelEnvironmentVariable:  "",
			},
			want: []string{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lookupEnv := func(name string) (string, bool) {
				value, ok := test.environment[name]
				return value, ok
			}

			if got := argumentsWithLoggingEnvironment(test.args, lookupEnv); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("argumentsWithLoggingEnvironment() = %q, want %q", got, test.want)
			}
		})
	}
}
