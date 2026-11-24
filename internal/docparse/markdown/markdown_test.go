package markdown

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProseSkipsCode(t *testing.T) {
	prose := proseOf(parseNodes(strings.Split("## Title\n\nUse `crm` here.\n\n```\n# crm configure\n```\n", "\n")))
	assert.NotContains(t, prose, "crm")
	assert.Contains(t, prose, "Use  here.")
}
