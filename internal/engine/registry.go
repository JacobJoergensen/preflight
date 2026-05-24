package engine

import (
	"github.com/JacobJoergensen/preflight/internal/ecosystem"
	"github.com/JacobJoergensen/preflight/internal/ecosystem/composer"
	"github.com/JacobJoergensen/preflight/internal/ecosystem/dotnet"
	"github.com/JacobJoergensen/preflight/internal/ecosystem/env"
	"github.com/JacobJoergensen/preflight/internal/ecosystem/golang"
	"github.com/JacobJoergensen/preflight/internal/ecosystem/js"
	"github.com/JacobJoergensen/preflight/internal/ecosystem/php"
	"github.com/JacobJoergensen/preflight/internal/ecosystem/python"
	"github.com/JacobJoergensen/preflight/internal/ecosystem/ruby"
	"github.com/JacobJoergensen/preflight/internal/ecosystem/rust"
	"github.com/JacobJoergensen/preflight/internal/exec"
)

var registeredSpecs = []*ecosystem.Spec{
	golang.Spec(),
	rust.Spec(),
	composer.Spec(),
	js.Spec(),
	js.NodeSpec(),
	python.Spec(),
	php.Spec(),
	ruby.Spec(),
	dotnet.Spec(),
	env.Spec(),
}

func init() {
	ecosystem.SetSpecs(registeredSpecs)
	exec.SetGate(ecosystem.Gate)
}
