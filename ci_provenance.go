package evals

import (
	"encoding/json"
	"os"
	"regexp"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel/attribute"
)

// ciProvenanceAttributes returns the OpenTelemetry `vcs.*` and `cicd.*`
// attributes for the CI run an experiment executes in, so its span can be
// matched to a commit, branch and pull request. GitHub Actions and GitLab CI are
// detected; provenance that can't be determined is omitted.
//
// PYDANTIC_EVALS_GIT_COMMIT_SHA and PYDANTIC_EVALS_GIT_BRANCH override the
// detected commit and branch, and also apply outside CI. Setting
// PYDANTIC_EVALS_CI_PROVENANCE=false disables all of it.
func ciProvenanceAttributes() []attribute.KeyValue {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("PYDANTIC_EVALS_CI_PROVENANCE"))) {
	case "0", "false", "no", "off":
		return nil
	}

	p := ciProvenance{}
	switch {
	case os.Getenv("GITHUB_ACTIONS") == "true":
		p.github()
	case os.Getenv("GITLAB_CI") == "true":
		p.gitlab()
	}
	if sha := os.Getenv("PYDANTIC_EVALS_GIT_COMMIT_SHA"); sha != "" {
		p.set(attrRevision, sha)
	}
	p.setHeadRef(os.Getenv("PYDANTIC_EVALS_GIT_BRANCH"), "branch")
	return p.attrs
}

const (
	attrProvider     = "vcs.provider.name"
	attrRepoURL      = "vcs.repository.url.full"
	attrRevision     = "vcs.ref.head.revision"
	attrHeadName     = "vcs.ref.head.name"
	attrHeadType     = "vcs.ref.head.type"
	attrBaseName     = "vcs.ref.base.name"
	attrBaseRevision = "vcs.ref.base.revision"
	attrChangeID     = "vcs.change.id"
	attrRunURL       = "cicd.pipeline.run.url.full"
)

// githubRefEvents are the GitHub Actions events whose GITHUB_SHA and GITHUB_REF
// are the code the workflow checks out by default. Others, like issue_comment,
// point them at the default branch while the workflow usually checks out a PR.
var githubRefEvents = map[string]bool{"push": true, "schedule": true, "workflow_dispatch": true, "release": true}

var pullRequestRefRE = regexp.MustCompile(`^refs/pull/(\d+)/`)

type ciProvenance struct {
	attrs []attribute.KeyValue
}

// set records key=value, replacing an earlier value for key; empty values are skipped.
func (p *ciProvenance) set(key, value string) {
	if value == "" {
		return
	}
	for i, kv := range p.attrs {
		if string(kv.Key) == key {
			p.attrs[i] = attribute.String(key, value)
			return
		}
	}
	p.attrs = append(p.attrs, attribute.String(key, value))
}

func (p *ciProvenance) setHeadRef(name, refType string) {
	if name != "" {
		p.set(attrHeadName, name)
		p.set(attrHeadType, refType)
	}
}

type githubPullRequest struct {
	Number *int `json:"number"`
	Head   struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	} `json:"head"`
	Base struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	} `json:"base"`
}

func readPullRequest(path string) *githubPullRequest {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var event struct {
		PullRequest *githubPullRequest `json:"pull_request"`
	}
	if json.Unmarshal(data, &event) != nil {
		return nil
	}
	return event.PullRequest
}

func (p *ciProvenance) github() {
	p.set(attrProvider, "github")
	if server, repo := os.Getenv("GITHUB_SERVER_URL"), os.Getenv("GITHUB_REPOSITORY"); server != "" && repo != "" {
		p.set(attrRepoURL, server+"/"+repo)
		if runID := os.Getenv("GITHUB_RUN_ID"); runID != "" {
			runURL := server + "/" + repo + "/actions/runs/" + runID
			// Each re-run of a workflow run is a separate attempt with its own URL.
			if attempt := os.Getenv("GITHUB_RUN_ATTEMPT"); attempt != "" {
				runURL += "/attempts/" + attempt
			}
			p.set(attrRunURL, runURL)
		}
	}

	eventName := os.Getenv("GITHUB_EVENT_NAME")
	headType := "branch"
	var commitSHA, headName, baseName, baseSHA, number string
	if pr := readPullRequest(os.Getenv("GITHUB_EVENT_PATH")); pr != nil {
		// On PR events GITHUB_SHA is the synthetic merge commit, so the head comes from the payload.
		commitSHA, headName, baseName, baseSHA = pr.Head.SHA, pr.Head.Ref, pr.Base.Ref, pr.Base.SHA
		if headName == "" {
			headName = os.Getenv("GITHUB_HEAD_REF")
		}
		if baseName == "" {
			baseName = os.Getenv("GITHUB_BASE_REF")
		}
		if pr.Number != nil {
			number = strconv.Itoa(*pr.Number)
		}
	} else if headRef := os.Getenv("GITHUB_HEAD_REF"); headRef != "" {
		// A PR event whose payload couldn't be read: GITHUB_SHA would be the merge commit, so omit the SHA.
		headName, baseName = headRef, os.Getenv("GITHUB_BASE_REF")
		if m := pullRequestRefRE.FindStringSubmatch(os.Getenv("GITHUB_REF")); m != nil {
			number = m[1]
		}
	} else if githubRefEvents[eventName] {
		commitSHA = os.Getenv("GITHUB_SHA")
		refType := os.Getenv("GITHUB_REF_TYPE")
		if refType == "branch" || refType == "tag" {
			headName = os.Getenv("GITHUB_REF_NAME")
		}
		if refType == "tag" {
			headType = "tag"
		}
	}

	if eventName == "pull_request_target" {
		// These runs check out the base branch by default, so the PR head may not be the code under test.
		commitSHA, headName = "", ""
	}

	p.set(attrRevision, commitSHA)
	p.setHeadRef(headName, headType)
	p.set(attrBaseName, baseName)
	p.set(attrBaseRevision, baseSHA)
	p.set(attrChangeID, number)
}

func (p *ciProvenance) gitlab() {
	p.set(attrProvider, "gitlab")
	p.set(attrRepoURL, os.Getenv("CI_PROJECT_URL"))
	p.set(attrRunURL, os.Getenv("CI_PIPELINE_URL"))

	if iid := os.Getenv("CI_MERGE_REQUEST_IID"); iid != "" {
		commitSHA := os.Getenv("CI_COMMIT_SHA")
		switch os.Getenv("CI_MERGE_REQUEST_EVENT_TYPE") {
		case "merged_result", "merge_train":
			// These pipelines build a merge commit; GitLab reports the source branch head separately.
			commitSHA = os.Getenv("CI_MERGE_REQUEST_SOURCE_BRANCH_SHA")
		}
		p.set(attrRevision, commitSHA)
		p.setHeadRef(os.Getenv("CI_MERGE_REQUEST_SOURCE_BRANCH_NAME"), "branch")
		p.set(attrBaseName, os.Getenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME"))
		p.set(attrBaseRevision, os.Getenv("CI_MERGE_REQUEST_TARGET_BRANCH_SHA"))
		p.set(attrChangeID, iid)
		return
	}
	p.set(attrRevision, os.Getenv("CI_COMMIT_SHA"))
	if tag := os.Getenv("CI_COMMIT_TAG"); tag != "" {
		p.setHeadRef(tag, "tag")
	} else {
		p.setHeadRef(os.Getenv("CI_COMMIT_BRANCH"), "branch")
	}
}
