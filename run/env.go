// Copyright 2022 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package run

import (
	"os"
	"sort"

	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/stack"
	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"
)

const (
	// ErrParsingCfg indicates that an error happened while parsing configuration.
	ErrParsingCfg errors.Kind = "parsing terramate.config.run.env configuration"

	// ErrLoadingGlobals indicates that an error happened while loading globals.
	ErrLoadingGlobals errors.Kind = "loading globals to evaluate terramate.config.run.env configuration"

	// ErrEval indicates that an error happened while evaluating one of the
	// terramate.config.run.env attributes.
	ErrEval errors.Kind = "evaluating terramate.config.run.env attribute"

	// ErrInvalidEnvVarType indicates the env var attribute
	// has an invalid type.
	ErrInvalidEnvVarType errors.Kind = "invalid environment variable type"
)

// EnvVars represents a set of environment variables to be used
// when running commands. Each string follows the same format used
// on os.Environ and can be used to set env on exec.Cmd.
type EnvVars []string

// LoadEnv will load environment variables to be exported when running any command
// inside the given stack. The order of the env vars is guaranteed to be the same
// and is ordered lexicographically.
func LoadEnv(rootdir string, st stack.S) (EnvVars, error) {
	logger := log.With().
		Str("action", "run.Env()").
		Str("root", rootdir).
		Stringer("stack", st).
		Logger()

	logger.Trace().Msg("parsing configuration")

	cfg, err := hcl.ParseDir(rootdir)
	if err != nil {
		return nil, errors.E(ErrParsingCfg, err)
	}

	logger.Trace().Msg("checking if we have run env config")

	if !cfg.HasRunEnv() {
		logger.Trace().Msg("no run env config found, nothing to do")
		return nil, nil
	}

	logger.Trace().Msg("loading globals")

	globals, err := stack.LoadGlobals(rootdir, st)
	if err != nil {
		return nil, errors.E(ErrLoadingGlobals, err)
	}

	evalctx := stack.NewEvalCtx(rootdir, st, globals)
	evalctx.SetEnv(os.Environ())

	envVars := EnvVars{}

	attrs := cfg.Terramate.Config.Run.Env.Attributes
	sort.Stable(attrs)

	for _, attribute := range attrs {
		hclattr := attribute.Value()
		logger = logger.With().
			Str("attribute", hclattr.Name).
			Logger()

		logger.Trace().Msg("evaluating")

		val, err := evalctx.Eval(hclattr.Expr)
		if err != nil {
			return nil, errors.E(ErrEval, hclattr.NameRange,
				err, "attribute origin %s", attribute.Origin())
		}

		logger.Trace().Msg("checking evaluated value type")

		if val.Type() != cty.String {
			return nil, errors.E(
				ErrInvalidEnvVarType,
				hclattr.NameRange,
				"attr has type %s but must be string, attribute origin %s",
				val.Type().FriendlyName(),
				attribute.Origin(),
			)
		}
		envVars = append(envVars, hclattr.Name+"="+val.AsString())

		logger.Trace().Msg("env var loaded")
	}

	return envVars, nil
}