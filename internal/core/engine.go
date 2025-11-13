package core

// Engine is the hidden build/run/test/deps abstraction used by JPM.
// Implementations: maven (phase 1), native (phase 2+), gradle (future).
// Users never interact with the engine directly; CLI translates to these calls.

type Engine interface {
	// Build compiles sources and packages artifacts according to manifest.
	Build(ctx Context, plan BuildPlan) (BuildResult, error)
	// Run executes a main class from the built artifact (or directly from classes).
	Run(ctx Context, run RunSpec) (int, error) // returns exit code
	// Test executes tests and returns aggregated results.
	Test(ctx Context, spec TestSpec) (TestResult, error)
	// Deps performs dependency operations (show/add/tree/audit) based on manifest.
	Deps(ctx Context, op DepsOp) (DepsResult, error)
}

// Context carries execution-time options, verbosity, env, and working dirs.
type Context struct {
	ProjectRoot string
	WorkDir     string // usually .jpm/work
	CacheDir    string // usually .jpm/cache
	LogsDir     string // usually .jpm/logs
	Env         map[string]string
	Verbose     bool
}

// BuildPlan describes what to build.
type BuildPlan struct {
	Manifest    Manifest // parsed jpm.yaml
	Targets     []string // e.g., ["jar"], future: ["jar","sources","javadoc"]
	Incremental bool     // attempt incremental build
}

// BuildResult summarizes outputs.
type BuildResult struct {
	Artifacts  []Artifact // produced artifacts
	Warnings   []string
	DurationMs int64
}

type Artifact struct {
	Path    string // absolute path to file
	Kind    string // jar, sources-jar, javadoc-jar
	MainCls string // detected/selected main class (if any)
}

// RunSpec configures execution.
type RunSpec struct {
	Manifest  Manifest
	MainClass string   // optional; engine may auto-detect
	Args      []string // program args
	JvmArgs   []string // VM args
}

// TestSpec configures tests.
type TestSpec struct {
	Manifest Manifest
	Filter   string // package/class/method filter
}

type TestResult struct {
	Passed    int
	Failed    int
	Skipped   int
	ReportDir string
}

// Deps operations

type DepsOp struct {
	Manifest Manifest
	Action   DepsAction
	Query    string      // for search/show
	Entry    *Dependency // for add/remove
}

type DepsAction string

const (
	DepsShow   DepsAction = "show"
	DepsTree   DepsAction = "tree"
	DepsAdd    DepsAction = "add"
	DepsRemove DepsAction = "remove"
)

type DepsResult struct {
	Deps     []Dependency
	Tree     *DependencyTree
	Changed  bool
	Warnings []string
}

// Manifest is the in-memory shape parsed from jpm.yaml (minimal for prototype).
// Extend as needed alongside the YAML schema.
type Manifest struct {
	Name      string
	BuildTool string `yaml:"build_tool"` // user-facing label (e.g., "maven")
	Engine    string // internal engine key (e.g., "maven", "native")
	Java      struct {
		Version string
	}
	Project struct {
		GroupID    string `yaml:"group_id"`
		ArtifactID string `yaml:"artifact_id"`
		Version    string
	}
	App struct {
		MainClass string `yaml:"main_class"`
	} `yaml:"app"`
	Dependencies []Dependency
}
