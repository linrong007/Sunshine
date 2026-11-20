package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// rawURLPatterns 把 provider 前缀映射到原始文件 URL 模板，
// 对应上游 RAW_URL_PATTERNS。
var rawURLPatterns = map[string]string{
	"gh":        "https://raw.githubusercontent.com/{repo}/{ref}/{subdir}",
	"github":    "https://raw.githubusercontent.com/{repo}/{ref}/{subdir}",
	"gitlab":    "https://gitlab.com/{repo}/-/raw/{ref}/{subdir}",
	"bitbucket": "https://bitbucket.org/{repo}/raw/{ref}/{subdir}",
}

// SupportedProviders 是支持的 provider 前缀。
var SupportedProviders = []string{"gh", "github", "gitlab", "bitbucket"}

// knownPublicDomains 是有专用前缀的公开域名。
var knownPublicDomains = []string{"github.com", "gitlab.com", "bitbucket.org"}

// publicDomainToPrefix 把公开域名映射到 giget 前缀。
var publicDomainToPrefix = map[string]string{
	"github.com":    "gh",
	"gitlab.com":    "gitlab",
	"bitbucket.org": "bitbucket",
}

var (
	httpsPrefixRes = []*regexp.Regexp{
		regexp.MustCompile(`^https?://github\.com/`),
		regexp.MustCompile(`^https?://gitlab\.com/`),
		regexp.MustCompile(`^https?://bitbucket\.org/`),
	}
	httpsPrefixes  = []string{"gh:", "gitlab:", "bitbucket:"}
	httpsTreeRe    = regexp.MustCompile(`^(.+?)(?:/-)?/tree/([^/]+)(?:/(.+?))?(?:\.git)?/?$`)
	scpSSHRe       = regexp.MustCompile(`^git@([^:]+):(.+?)(?:\.git)?/?$`)
	protocolSSHRe  = regexp.MustCompile(`^ssh://git@([^/:]+)(?::(\d+))?/(.+?)(?:\.git)?/?$`)
	httpsUnknownRe = regexp.MustCompile(`^https?://([^/]+)/(.+?)(?:\.git)?/?$`)
	gitlabTreeRe   = regexp.MustCompile(`^(.+?)(?:/-)?/tree/([^/]+)(?:/(.+?))?$`)
	refSplitRe     = regexp.MustCompile(`^([^#]+?)(?:#(.+))?$`)
)

// NormalizeRegistrySource 把 HTTPS URL 转成 giget 风格的来源字符串，
// 如 https://github.com/user/repo → gh:user/repo，
// https://github.com/user/repo/tree/branch/path → gh:user/repo/path#branch。
// 非已知的 HTTPS URL 原样返回。对应上游 normalizeRegistrySource。
func NormalizeRegistrySource(source string) string {
	for i, re := range httpsPrefixRes {
		if !re.MatchString(source) {
			continue
		}
		path := re.ReplaceAllString(source, "")
		if m := httpsTreeRe.FindStringSubmatch(path); m != nil {
			repo, ref, subdir := m[1], m[2], m[3]
			if subdir != "" {
				return fmt.Sprintf("%s%s/%s#%s", httpsPrefixes[i], repo, subdir, ref)
			}
			return fmt.Sprintf("%s%s#%s", httpsPrefixes[i], repo, ref)
		}
		cleaned := strings.TrimSuffix(strings.TrimSuffix(path, "/"), ".git")
		cleaned = strings.TrimSuffix(cleaned, "/")
		return httpsPrefixes[i] + cleaned
	}
	return source
}

type gitURLInput struct {
	provider string
	repo     string
	kind     SourceKind
	host     string
	sshHost  string
	sshPort  string
	sshStyle string // "scp" 或 "ssh-url"
	// scheme 是自建 http/https 来源的协议（"http"/"https"，默认 "https"）；
	// 仅 host 非空时生效，giget 前缀与公开域名不走这条，仍强制 https。
	scheme string
}

// buildGitURL 拼出 go-git 克隆用的远端地址，对应上游 buildRegistryGitUrl。
func buildGitURL(in gitURLInput) string {
	if in.kind == SourceKindSSH && in.sshHost != "" {
		if in.sshStyle == "ssh-url" {
			port := ""
			if in.sshPort != "" {
				port = ":" + in.sshPort
			}
			return fmt.Sprintf("ssh://git@%s%s/%s.git", in.sshHost, port, in.repo)
		}
		return fmt.Sprintf("git@%s:%s.git", in.sshHost, in.repo)
	}
	if in.host != "" {
		scheme := in.scheme
		if scheme == "" {
			scheme = "https"
		}
		return fmt.Sprintf("%s://%s/%s.git", scheme, in.host, in.repo)
	}
	switch in.provider {
	case "gh", "github":
		return fmt.Sprintf("https://github.com/%s.git", in.repo)
	case "gitlab":
		return fmt.Sprintf("https://gitlab.com/%s.git", in.repo)
	case "bitbucket":
		return fmt.Sprintf("https://bitbucket.org/%s.git", in.repo)
	default:
		return in.repo
	}
}

// parseLocalSource 识别本地 git 仓库路径（Go 版补充来源，上游无此用法）：
// 已存在的目录，或 file:// URL。路径可以是仓库根，也可以是仓库内的
// 子目录（此时向上找到 enclosing 的 .git 所在目录为仓库根，
// 剩余部分为 subdir，与 gh:owner/repo/subdir 的语义一致）。
// 路径存在但不在任何 git 仓库里时，仍按本地来源返回，克隆阶段会
// 报“不是有效的 git 仓库”。
func parseLocalSource(source string) *Source {
	path := source
	if strings.HasPrefix(path, "file://") {
		path = strings.TrimPrefix(path, "file://")
	} else {
		// 带冒号的都不是本地路径（gh:、C:\、URL 等）；只接受
		// 绝对路径、./ 与 ../ 开头的相对路径。
		if strings.Contains(path, ":") {
			return nil
		}
		if !filepath.IsAbs(path) && !strings.HasPrefix(path, ".") {
			return nil
		}
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil
	}
	st, err := os.Stat(abs)
	if err != nil || !st.IsDir() {
		return nil
	}
	repoRoot, subdir := splitLocalRepo(abs)
	return &Source{
		Provider:    "local",
		Repo:        filepath.Base(repoRoot),
		Subdir:      subdir,
		Ref:         "",
		GigetSource: source,
		GitURL:      repoRoot,
		PreferGit:   true,
		Kind:        SourceKindLocal,
		Original:    source,
	}
}

// splitLocalRepo 把本地路径拆成仓库根与子目录：从 path 自身向上找
// 第一个含 .git（目录或 worktree 的 .git 文件）的目录；找不到时
// 整个 path 当仓库根（克隆时会报错）。
func splitLocalRepo(path string) (root, subdir string) {
	for dir := path; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			rel, err := filepath.Rel(dir, path)
			if err != nil {
				return dir, ""
			}
			if rel == "." {
				return dir, ""
			}
			return dir, filepath.ToSlash(rel)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return path, ""
		}
	}
}

// hasExplicitGitSuffix 判断来源是否以 .git 结尾：先切掉 #ref、再去掉末尾
// 的 /，剩余部分以 .git 结尾即为真。三个正则（scpSSHRe、protocolSSHRe、
// httpsUnknownRe）里的 (?:\.git)? 会把后缀吃掉，所以必须在正则处理之前
// 对原始来源判断并记下标志，再传到切段逻辑。
// 以 .git 结尾是用户明确声明“这就是仓库路径”的信号：整个路径都是仓库，
// subdir 为空；不带 .git 时保持现有两段式规则。
func hasExplicitGitSuffix(source string) bool {
	s := source
	if i := strings.Index(s, "#"); i != -1 {
		s = s[:i]
	}
	s = strings.TrimSuffix(s, "/")
	return strings.HasSuffix(s, ".git")
}

// ParseRegistrySource 把 --registry 参数解析成结构化来源。
//
// 支持的输入（对应上游 parseRegistrySource，另加本地路径）：
//
//	| 格式                              | 例子                                     |
//	|-----------------------------------|------------------------------------------|
//	| giget 前缀                        | gh:org/repo、gitlab:org/repo#ref         |
//	| 公开 HTTPS                        | https://github.com/org/repo              |
//	| 公开 SSH                          | git@github.com:org/repo                  |
//	| 自建 HTTPS                        | https://git.corp.com/org/repo            |
//	| 自建 SSH                          | git@git.corp.com:org/repo                |
//	| ssh://（可带端口）               | ssh://git@host:2222/org/repo             |
//	| HTTPS（带端口）                   | https://host:8443/org/repo               |
//	| GitLab 浏览 URL                   | https://host/org/repo/-/tree/branch/path |
//	| 本地 git 仓库（Go 版补充）        | /path/to/repo、./repo、file:///path/repo |
//
// Ref 默认 main。未知域名默认按 GitLab 的 URL 形态处理（覆盖自建 GitLab）。
// provider 不支持时返回错误。
func ParseRegistrySource(source string) (*Source, error) {
	if strings.TrimSpace(source) == "" {
		return nil, NewError(ErrKindUnknown, "registry 来源不能为空")
	}
	if local := parseLocalSource(source); local != nil {
		return local, nil
	}
	// .git 后缀要在正则吃掉它之前判断（见 hasExplicitGitSuffix）。
	dotGit := hasExplicitGitSuffix(source)

	var host string
	var normalizedInput string
	kind := SourceKindPrefixed
	var sshHost, sshPort, sshStyle string
	// scheme 只对 httpsUnknownRe 匹配到的自建 http/https URL 取值；
	// giget 前缀与公开域名不走这条，默认 https。
	scheme := "https"
	// GitLab 多段仓库（子群组）切分提示（C2）：repoSegs>0 时仓库取
	// 前 repoSegs 段（tree 形式按 /tree/ 或 /-/tree/ 之前的段数定）；
	// fullRepo 为真时整个路径都是仓库、subdir 为空（GitLab 裸 HTTPS
	// 路径没有子目录分隔符，group/sub/repo 整体是仓库路径）。
	// giget 前缀与 SSH 形式仍按两段式走（前缀形式自带子目录语义，
	// 既有单测钉住），GitHub/Bitbucket 仓库恒为两段也不受影响。
	var repoSegs int
	var fullRepo bool

	// SSH：git@host:org/repo[.git] 或 ssh://git@host[:port]/org/repo[.git]
	if m := scpSSHRe.FindStringSubmatch(source); m != nil {
		kind = SourceKindSSH
		sshStyle = "scp"
		sshHost = m[1]
		sshPath := m[2]
		if prefix, ok := publicDomainToPrefix[sshHost]; ok {
			normalizedInput = prefix + ":" + sshPath
		} else {
			host = sshHost
			normalizedInput = "gitlab:" + sshPath
		}
	} else if m := protocolSSHRe.FindStringSubmatch(source); m != nil {
		kind = SourceKindSSH
		sshStyle = "ssh-url"
		sshHost = m[1]
		sshPort = m[2]
		sshPath := m[3]
		if prefix, ok := publicDomainToPrefix[sshHost]; ok {
			normalizedInput = prefix + ":" + sshPath
		} else {
			host = sshHost
			normalizedInput = "gitlab:" + sshPath
		}
	}

	// 未知域名的 HTTPS URL（非 github/gitlab/bitbucket）。
	if normalizedInput == "" {
		if m := httpsUnknownRe.FindStringSubmatch(source); m != nil {
			kind = SourceKindHTTPS
			domain, pathPart := m[1], m[2]
			known := false
			for _, d := range knownPublicDomains {
				if d == domain {
					known = true
					break
				}
			}
			if !known {
				host = domain
				if strings.HasPrefix(source, "http://") {
					scheme = "http"
				}
				if tm := gitlabTreeRe.FindStringSubmatch(pathPart); tm != nil {
					repoPath, ref, subdir := tm[1], tm[2], tm[3]
					repoSegs = len(strings.Split(repoPath, "/"))
					normalizedInput = "gitlab:" + repoPath
					if subdir != "" {
						normalizedInput += "/" + subdir
					}
					normalizedInput += "#" + ref
				} else {
					normalizedInput = "gitlab:" + pathPart
					fullRepo = true
				}
			}
		}
	}

	// 公开 HTTPS（走 NormalizeRegistrySource 兜底）时同样定切分：
	// tree 形式按 /tree/（或 GitLab 的 /-/tree/）之前的段数定仓库；
	// gitlab.com 裸路径全段是仓库（子群组）。
	if normalizedInput == "" {
		for i, re := range httpsPrefixRes {
			if !re.MatchString(source) {
				continue
			}
			path := re.ReplaceAllString(source, "")
			if m := httpsTreeRe.FindStringSubmatch(path); m != nil {
				repoSegs = len(strings.Split(m[1], "/"))
			} else if httpsPrefixes[i] == "gitlab:" {
				fullRepo = true
			}
			break
		}
	}

	normalized := normalizedInput
	if normalized == "" {
		normalized = NormalizeRegistrySource(source)
	}

	colon := strings.Index(normalized, ":")
	if colon == -1 {
		return nil, NewError(ErrKindUnknown, "registry 来源 %q 格式无效，应为 gh:user/repo/path 形式", source)
	}
	provider := normalized[:colon]
	rest := normalized[colon+1:]

	pattern, ok := rawURLPatterns[provider]
	if !ok {
		return nil, NewError(ErrKindUnknown, "不支持的 provider %q，支持：%s", provider, strings.Join(SupportedProviders, "、"))
	}

	m := refSplitRe.FindStringSubmatch(rest)
	if m == nil {
		return nil, NewError(ErrKindUnknown, "registry 来源 %q 格式无效，应为 %s:user/repo/path 形式", normalized, provider)
	}
	pathPart, ref := m[1], m[2]
	if ref == "" {
		ref = "main"
	}

	var segments []string
	for _, seg := range strings.Split(pathPart, "/") {
		if seg != "" {
			segments = append(segments, seg)
		}
	}
	if len(segments) < 2 {
		return nil, NewError(ErrKindUnknown, "registry 来源 %q 格式无效，至少需要 user/repo", normalized)
	}
	repo := segments[0] + "/" + segments[1]
	subdir := strings.Join(segments[2:], "/")
	// 显式 .git 规则（仅自建来源，host 非空才算自建；giget 前缀与公开
	// 域名仍按两段式走）：整个路径就是仓库，subdir 为空。Join 后再去掉
	// .git——带 #ref 时正则吃不到后缀，最后一段会仍带 .git。
	if dotGit && host != "" {
		repo = strings.TrimSuffix(strings.Join(segments, "/"), ".git")
		subdir = ""
	} else if fullRepo {
		// GitLab 裸 HTTPS 路径：全段是仓库（子群组），无子目录。
		repo = strings.Join(segments, "/")
		subdir = ""
	} else if repoSegs >= 2 && repoSegs <= len(segments) {
		// tree 形式：/tree/ 之前的全段是仓库，其后是子目录。
		repo = strings.Join(segments[:repoSegs], "/")
		subdir = strings.Join(segments[repoSegs:], "/")
	}

	rawBaseURL := strings.ReplaceAll(pattern, "{repo}", repo)
	rawBaseURL = strings.ReplaceAll(rawBaseURL, "{ref}", ref)
	rawBaseURL = strings.ReplaceAll(rawBaseURL, "{subdir}", subdir)
	rawBaseURL = strings.TrimSuffix(rawBaseURL, "/")
	if host != "" && provider == "gitlab" {
		rawBaseURL = strings.Replace(rawBaseURL, "https://gitlab.com", scheme+"://"+host, 1)
	}

	gitURL := buildGitURL(gitURLInput{
		provider: provider,
		repo:     repo,
		kind:     kind,
		host:     host,
		sshHost:  sshHost,
		sshPort:  sshPort,
		sshStyle: sshStyle,
		scheme:   scheme,
	})

	return &Source{
		Provider:    provider,
		Repo:        repo,
		Subdir:      subdir,
		Ref:         ref,
		RawBaseURL:  rawBaseURL,
		GigetSource: normalized,
		Host:        host,
		GitURL:      gitURL,
		PreferGit:   kind == SourceKindSSH || host != "",
		Kind:        kind,
		Original:    source,
	}, nil
}
