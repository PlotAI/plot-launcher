package repository

import (
	"regexp"
	"strings"
)

// remotePattern matches the GitHub remote forms that name a repository. The
// first group is everything before the repository path, the second the owner,
// and the third the repository name.
var remotePattern = regexp.MustCompile(`(?i)^(https?://(?:www\.)?github\.com/|ssh://git@github\.com/|git@github\.com:)([^/]+)/([^/]+?)(?:\.git)?/?$`)

// remoteLocation splits a GitHub remote URL, so two spellings of the same
// repository can be compared. A URL that does not name a GitHub repository is
// not recognized.
func remoteLocation(rawURL string) (prefix, owner, name string, ok bool) {
	matches := remotePattern.FindStringSubmatch(strings.TrimSpace(rawURL))
	if matches == nil {
		return "", "", "", false
	}
	return matches[1], matches[2], matches[3], true
}

// sameRepository reports whether two remote URLs name the same GitHub
// repository, whatever transport and spelling they use.
func sameRepository(first, second string) bool {
	_, firstOwner, firstName, firstOk := remoteLocation(first)
	_, secondOwner, secondName, secondOk := remoteLocation(second)
	if !firstOk || !secondOk {
		return false
	}
	return strings.EqualFold(firstOwner, secondOwner) && strings.EqualFold(firstName, secondName)
}

// movedRemote returns the URL to store for a working directory whose
// repository moved to requested. The transport of current is kept, so an
// existing SSH remote keeps authenticating after the move.
func movedRemote(current, requested string) string {
	prefix, _, _, currentOk := remoteLocation(current)
	_, owner, name, requestedOk := remoteLocation(requested)
	if !currentOk || !requestedOk {
		return requested
	}
	return prefix + owner + "/" + name + ".git"
}
