module github.com/reflog/mattermost-plugin-silencer

go 1.12

require (
	github.com/blang/semver v3.6.1+incompatible // indirect
	github.com/mattermost/mattermost-server v1.4.1-0.20191122150651-5c41c8b1733b
	github.com/pkg/errors v0.8.1
)

// Workaround for https://github.com/golang/go/issues/30831 and fallout.
//replace github.com/golang/lint => github.com/golang/lint v0.0.0-20190227174305-8f45f776aaf1

replace willnorris.com/go/imageproxy => willnorris.com/go/imageproxy v0.8.1-0.20190422234945-d4246a08fdec
