// Package recipe provides a lexer and parser for .mozza recipe files.
package recipe

import "fmt"

// TokenType classifies a lexical token.
type TokenType int

const (
	// TokenComment represents a comment line starting with #.
	TokenComment TokenType = iota
	// TokenApp represents the "App:" header.
	TokenApp
	// TokenSectionHeader represents a section header (e.g., "Storefront:").
	TokenSectionHeader
	// TokenFrom represents the "from" keyword in directives.
	TokenFrom
	// TokenImage represents the "image" keyword in directives.
	TokenImage
	// TokenOpen represents the "open" keyword in directives.
	TokenOpen
	// TokenTo represents the "to" keyword.
	TokenTo
	// TokenThe represents the "the" keyword.
	TokenThe
	// TokenPublic represents the "public" keyword.
	TokenPublic
	// TokenOn represents the "on" keyword.
	TokenOn
	// TokenPort represents the "port" keyword.
	TokenPort
	// TokenHealth represents the "health" keyword.
	TokenHealth
	// TokenCheck represents the "check" keyword.
	TokenCheck
	// TokenRun represents the "run" keyword.
	TokenRun
	// TokenCopies represents the "copies" keyword.
	TokenCopies
	// TokenCopy represents the "copy" keyword.
	TokenCopy
	// TokenNeeds represents the "needs" keyword.
	TokenNeeds
	// TokenAnd represents the "and" keyword.
	TokenAnd
	// TokenComma represents a comma separator.
	TokenComma
	// TokenPostgres represents the "postgres" engine keyword.
	TokenPostgres
	// TokenMysql represents the "mysql" engine keyword.
	TokenMysql
	// TokenMongo represents the "mongo" engine keyword.
	TokenMongo
	// TokenRedis represents the "redis" engine keyword.
	TokenRedis
	// TokenMemcached represents the "memcached" engine keyword.
	TokenMemcached
	// TokenDaily represents the "daily" keyword.
	TokenDaily
	// TokenBackups represents the "backups" keyword.
	TokenBackups
	// TokenSet represents the "set" keyword for environment variables.
	TokenSet
	// TokenLimit represents the "limit" keyword for resource limits.
	TokenLimit
	// TokenCpu represents the "cpu" keyword in limit directives.
	TokenCpu
	// TokenMemory represents the "memory" keyword in limit directives.
	TokenMemory
	// TokenRestart represents the "restart" keyword.
	TokenRestart
	// TokenDomain represents the "domain" keyword.
	TokenDomain
	// TokenSecret represents the "secret" keyword.
	TokenSecret
	// TokenPull represents the "pull" keyword.
	TokenPull
	// TokenKey represents the "key" keyword.
	TokenKey
	// TokenImages represents the "Images:" header keyword.
	TokenImages
	// TokenCRDs represents the "CRDs:" header keyword.
	TokenCRDs
	// TokenNamespace represents the "Namespace:" header keyword.
	TokenNamespace
	// TokenAlways represents the "always" restart policy.
	TokenAlways
	// TokenUnlessStopped represents the "unless-stopped" restart policy.
	TokenUnlessStopped
	// TokenAs represents the "as" keyword (multi-port aliasing).
	TokenAs
	// TokenUsing represents the "using" keyword (multi-port protocol).
	TokenUsing
	// TokenEvery represents the "every" keyword (schedule interval).
	TokenEvery
	// TokenAt represents the "at" keyword (schedule time).
	TokenAt
	// TokenSchedule represents the "schedule" keyword.
	TokenSchedule
	// TokenOnce represents the "once" keyword (run modifier).
	TokenOnce
	// TokenCompletion represents the "completion" keyword (run modifier).
	TokenCompletion
	// TokenParallel represents the "parallel" keyword (run modifier).
	TokenParallel
	// TokenRetry represents the "retry" keyword (run modifier).
	TokenRetry
	// TokenNode represents the "node" keyword (daemon placement).
	TokenNode
	// TokenLabeled represents the "labeled" keyword (daemon selector).
	TokenLabeled
	// TokenExcept represents the "except" keyword (daemon exclusion).
	TokenExcept
	// TokenEach represents the "each" keyword (stateful sets).
	TokenEach
	// TokenOwn represents the "own" keyword (stateful volume ownership).
	TokenOwn
	// TokenOrdered represents the "ordered" keyword (stateful ordering).
	TokenOrdered
	// TokenFind represents the "find" keyword (stateful discovery).
	TokenFind
	// TokenBefore represents the "before" keyword (init container).
	TokenBefore
	// TokenStarting represents the "starting" keyword (init container).
	TokenStarting
	// TokenSidecar represents the "sidecar" keyword.
	TokenSidecar
	// TokenWith represents the "with" keyword.
	TokenWith
	// TokenMount represents the "mount" keyword (volume mounts).
	TokenMount
	// TokenFile represents the "file" keyword (config file mount).
	TokenFile
	// TokenConfig represents the "config" keyword (config mount).
	TokenConfig
	// TokenPermission represents the "permission" keyword.
	TokenPermission
	// TokenManage represents the "manage" keyword (RBAC).
	TokenManage
	// TokenAccount represents the "account" keyword (service account).
	TokenAccount
	// TokenRead represents the "read" keyword (permissions).
	TokenRead
	// TokenWrite represents the "write" keyword (permissions).
	TokenWrite
	// TokenReadOnly represents the "read-only" keyword (filesystem).
	TokenReadOnly
	// TokenReadiness represents the "readiness" keyword (probe).
	TokenReadiness
	// TokenLiveness represents the "liveness" keyword (probe).
	TokenLiveness
	// TokenStartup represents the "startup" keyword (probe).
	TokenStartup
	// TokenRunning represents the "running" keyword (probe status).
	TokenRunning
	// TokenStopping represents the "stopping" keyword (lifecycle).
	TokenStopping
	// TokenWait represents the "wait" keyword (lifecycle).
	TokenWait
	// TokenSeconds represents the "seconds" keyword (lifecycle duration).
	TokenSeconds
	// TokenPrefer represents the "prefer" keyword (scheduling).
	TokenPrefer
	// TokenRequire represents the "require" keyword (scheduling).
	TokenRequire
	// TokenAvoid represents the "avoid" keyword (scheduling).
	TokenAvoid
	// TokenSpread represents the "spread" keyword (scheduling).
	TokenSpread
	// TokenAcross represents the "across" keyword (scheduling).
	TokenAcross
	// TokenZones represents the "zones" keyword (scheduling).
	TokenZones
	// TokenNever represents the "never" keyword (scheduling).
	TokenNever
	// TokenAccept represents the "accept" keyword (network policy).
	TokenAccept
	// TokenTraffic represents the "traffic" keyword (network policy).
	TokenTraffic
	// TokenBlock represents the "block" keyword (network policy).
	TokenBlock
	// TokenReachable represents the "reachable" keyword (network policy).
	TokenReachable
	// TokenKind represents the "kind" keyword.
	TokenKind
	// TokenScale represents the "scale" keyword (auto-scaling).
	TokenScale
	// TokenBetween represents the "between" keyword (auto-scaling range).
	TokenBetween
	// TokenBased represents the "based" keyword (auto-scaling metric).
	TokenBased
	// TokenKeep represents the "keep" keyword (disruption budget).
	TokenKeep
	// TokenDuring represents the "during" keyword (disruption budget).
	TokenDuring
	// TokenUpdates represents the "updates" keyword (disruption budget).
	TokenUpdates
	// TokenAllow represents the "allow" keyword (disruption budget).
	TokenAllow
	// TokenDown represents the "down" keyword (disruption budget).
	TokenDown
	// TokenUser represents the "user" keyword (security context).
	TokenUser
	// TokenGroup represents the "group" keyword (security context).
	TokenGroup
	// TokenDrop represents the "drop" keyword (security capabilities).
	TokenDrop
	// TokenCapability represents the "capability" keyword (security).
	TokenCapability
	// TokenFilesystem represents the "filesystem" keyword (security).
	TokenFilesystem
	// TokenUpdate represents the "update" keyword (rolling update).
	TokenUpdate
	// TokenGraceful represents the "graceful" keyword (shutdown).
	TokenGraceful
	// TokenShutdown represents the "shutdown" keyword (graceful stop).
	TokenShutdown

	// TokenString represents a quoted string value.
	TokenString
	// TokenNumber represents an integer value.
	TokenNumber
	// TokenBool represents a boolean value (true/false).
	TokenBool
	// TokenIdent represents an unquoted identifier or value.
	TokenIdent
	// TokenNewline represents a line boundary.
	TokenNewline
	// TokenEOF represents the end of input.
	TokenEOF
)

// tokenTypeNames maps each TokenType to its human-readable name.
var tokenTypeNames = [...]string{ //nolint:gochecknoglobals // immutable lookup table
	TokenComment:       "Comment",
	TokenApp:           "App",
	TokenSectionHeader: "SectionHeader",
	TokenFrom:          "From",
	TokenImage:         "Image",
	TokenOpen:          "Open",
	TokenTo:            "To",
	TokenThe:           "The",
	TokenPublic:        "Public",
	TokenOn:            "On",
	TokenPort:          "Port",
	TokenHealth:        "Health",
	TokenCheck:         "Check",
	TokenRun:           "Run",
	TokenCopies:        "Copies",
	TokenCopy:          "Copy",
	TokenNeeds:         "Needs",
	TokenAnd:           "And",
	TokenComma:         "Comma",
	TokenPostgres:      "Postgres",
	TokenMysql:         "Mysql",
	TokenMongo:         "Mongo",
	TokenRedis:         "Redis",
	TokenMemcached:     "Memcached",
	TokenDaily:         "Daily",
	TokenBackups:       "Backups",
	TokenSet:           "Set",
	TokenLimit:         "Limit",
	TokenCpu:           "Cpu",
	TokenMemory:        "Memory",
	TokenRestart:       "Restart",
	TokenDomain:        "Domain",
	TokenSecret:        "Secret",
	TokenPull:          "Pull",
	TokenKey:           "Key",
	TokenImages:        "Images",
	TokenCRDs:          "CRDs",
	TokenNamespace:     "Namespace",
	TokenAlways:        "Always",
	TokenUnlessStopped: "UnlessStopped",
	TokenAs:            "As",
	TokenUsing:         "Using",
	TokenEvery:         "Every",
	TokenAt:            "At",
	TokenSchedule:      "Schedule",
	TokenOnce:          "Once",
	TokenCompletion:    "Completion",
	TokenParallel:      "Parallel",
	TokenRetry:         "Retry",
	TokenNode:          "Node",
	TokenLabeled:       "Labeled",
	TokenExcept:        "Except",
	TokenEach:          "Each",
	TokenOwn:           "Own",
	TokenOrdered:       "Ordered",
	TokenFind:          "Find",
	TokenBefore:        "Before",
	TokenStarting:      "Starting",
	TokenSidecar:       "Sidecar",
	TokenWith:          "With",
	TokenMount:         "Mount",
	TokenFile:          "File",
	TokenConfig:        "Config",
	TokenPermission:    "Permission",
	TokenManage:        "Manage",
	TokenAccount:       "Account",
	TokenRead:          "Read",
	TokenWrite:         "Write",
	TokenReadOnly:      "ReadOnly",
	TokenReadiness:     "Readiness",
	TokenLiveness:      "Liveness",
	TokenStartup:       "Startup",
	TokenRunning:       "Running",
	TokenStopping:      "Stopping",
	TokenWait:          "Wait",
	TokenSeconds:       "Seconds",
	TokenPrefer:        "Prefer",
	TokenRequire:       "Require",
	TokenAvoid:         "Avoid",
	TokenSpread:        "Spread",
	TokenAcross:        "Across",
	TokenZones:         "Zones",
	TokenNever:         "Never",
	TokenAccept:        "Accept",
	TokenTraffic:       "Traffic",
	TokenBlock:         "Block",
	TokenReachable:     "Reachable",
	TokenKind:          "Kind",
	TokenScale:         "Scale",
	TokenBetween:       "Between",
	TokenBased:         "Based",
	TokenKeep:          "Keep",
	TokenDuring:        "During",
	TokenUpdates:       "Updates",
	TokenAllow:         "Allow",
	TokenDown:          "Down",
	TokenUser:          "User",
	TokenGroup:         "Group",
	TokenDrop:          "Drop",
	TokenCapability:    "Capability",
	TokenFilesystem:    "Filesystem",
	TokenUpdate:        "Update",
	TokenGraceful:      "Graceful",
	TokenShutdown:      "Shutdown",
	TokenString:        "String",
	TokenNumber:        "Number",
	TokenBool:          "Bool",
	TokenIdent:         "Ident",
	TokenNewline:       "Newline",
	TokenEOF:           "EOF",
}

// String returns the human-readable name of the token type.
func (t TokenType) String() string {
	if int(t) < len(tokenTypeNames) {
		return tokenTypeNames[t]
	}

	return fmt.Sprintf("TokenType(%d)", int(t))
}

// keywords maps keyword strings to their corresponding token types.
var keywords = map[string]TokenType{ //nolint:gochecknoglobals // immutable lookup table
	"from":           TokenFrom,
	"image":          TokenImage,
	"open":           TokenOpen,
	"to":             TokenTo,
	"the":            TokenThe,
	"public":         TokenPublic,
	"on":             TokenOn,
	"port":           TokenPort,
	"health":         TokenHealth,
	"check":          TokenCheck,
	"run":            TokenRun,
	"copies":         TokenCopies,
	"copy":           TokenCopy,
	"needs":          TokenNeeds,
	"and":            TokenAnd,
	"daily":          TokenDaily,
	"backups":        TokenBackups,
	"set":            TokenSet,
	"limit":          TokenLimit,
	"cpu":            TokenCpu,
	"memory":         TokenMemory,
	"restart":        TokenRestart,
	"domain":         TokenDomain,
	"secret":         TokenSecret,
	"pull":           TokenPull,
	"key":            TokenKey,
	"always":         TokenAlways,
	"unless-stopped": TokenUnlessStopped,
	"postgres":       TokenPostgres,
	"mysql":          TokenMysql,
	"mongo":          TokenMongo,
	"redis":          TokenRedis,
	"memcached":      TokenMemcached,
	"as":             TokenAs,
	"using":          TokenUsing,
	"every":          TokenEvery,
	"at":             TokenAt,
	"schedule":       TokenSchedule,
	"once":           TokenOnce,
	"completion":     TokenCompletion,
	"parallel":       TokenParallel,
	"retry":          TokenRetry,
	"node":           TokenNode,
	"labeled":        TokenLabeled,
	"except":         TokenExcept,
	"each":           TokenEach,
	"own":            TokenOwn,
	"ordered":        TokenOrdered,
	"find":           TokenFind,
	"before":         TokenBefore,
	"starting":       TokenStarting,
	"sidecar":        TokenSidecar,
	"with":           TokenWith,
	"mount":          TokenMount,
	"file":           TokenFile,
	"config":         TokenConfig,
	"permission":     TokenPermission,
	"manage":         TokenManage,
	"account":        TokenAccount,
	"read":           TokenRead,
	"write":          TokenWrite,
	"read-only":      TokenReadOnly,
	"readiness":      TokenReadiness,
	"liveness":       TokenLiveness,
	"startup":        TokenStartup,
	"running":        TokenRunning,
	"stopping":       TokenStopping,
	"wait":           TokenWait,
	"seconds":        TokenSeconds,
	"prefer":         TokenPrefer,
	"require":        TokenRequire,
	"avoid":          TokenAvoid,
	"spread":         TokenSpread,
	"across":         TokenAcross,
	"zones":          TokenZones,
	"never":          TokenNever,
	"accept":         TokenAccept,
	"traffic":        TokenTraffic,
	"block":          TokenBlock,
	"reachable":      TokenReachable,
	"kind":           TokenKind,
	"scale":          TokenScale,
	"between":        TokenBetween,
	"based":          TokenBased,
	"keep":           TokenKeep,
	"during":         TokenDuring,
	"updates":        TokenUpdates,
	"allow":          TokenAllow,
	"down":           TokenDown,
	"user":           TokenUser,
	"group":          TokenGroup,
	"drop":           TokenDrop,
	"capability":     TokenCapability,
	"filesystem":     TokenFilesystem,
	"update":         TokenUpdate,
	"graceful":       TokenGraceful,
	"shutdown":       TokenShutdown,
}

// Token represents a lexical token with its position and value.
type Token struct {
	Type  TokenType
	Value string
	Line  int // 1-based line number.
	Col   int // 1-based column number.
}

// String returns a debug representation of the token.
func (t Token) String() string {
	return fmt.Sprintf("%s(%q)@%d:%d", t.Type, t.Value, t.Line, t.Col)
}
