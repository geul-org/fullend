package orchestrator

import (
	ssacgenerator "github.com/geul-org/ssac/generator"
	stmlgenerator "github.com/geul-org/stml/generator"
)

// TargetProfile defines the backend + frontend code generation targets.
type TargetProfile struct {
	Backend  ssacgenerator.Target
	Frontend stmlgenerator.Target
}

// DefaultProfile returns Go backend + React frontend.
func DefaultProfile() *TargetProfile {
	return &TargetProfile{
		Backend:  ssacgenerator.DefaultTarget(),
		Frontend: stmlgenerator.DefaultTarget(),
	}
}
