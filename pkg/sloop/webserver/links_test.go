/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package webserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_makeExternalLinks_SimpleCaseOneUrl(t *testing.T) {
	linkTemplates := []ResourceLinkTemplate{
		{
			Text:        "ThisName1",
			UrlTemplate: "http://somehost/{{.Namespace}}/{{.Name}}/{{.Kind}}",
			Kinds:       []string{"someKind"},
		},
		{
			Text:        "ThisName2",
			UrlTemplate: "doesNotMatter",
			Kinds:       []string{"someOther", "abc"},
		},
		{
			Text:        "ThisName3",
			UrlTemplate: "http://urlforallkinds/",
			Kinds:       []string{},
		},
	}
	links, err := makeResourceLinks("someNamespace", "someName", "someKind", linkTemplates)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(links))
	assert.Equal(t, "ThisName1", links[0].Text)
	assert.Equal(t, "http://somehost/someNamespace/someName/someKind", links[0].Url)
	assert.Equal(t, "ThisName3", links[1].Text)
	assert.Equal(t, "http://urlforallkinds/", links[1].Url)
}

func Test_makeExternalLinks_UpperAndLowerWork(t *testing.T) {
	linkTemplates := []ResourceLinkTemplate{
		{Text: "ThisName",
			UrlTemplate: "http://somehost/{{.Namespace | ToUpper}}/{{.Name | ToLower}}"},
	}
	links, err := makeResourceLinks("someNamespace", "someName", "someKind", linkTemplates)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(links))
	assert.Equal(t, "ThisName", links[0].Text)
	assert.Equal(t, "http://somehost/SOMENAMESPACE/somename", links[0].Url)
}

func Test_makeExternalLinks_EmptyReturnsNothing(t *testing.T) {
	linkTemplates := []ResourceLinkTemplate{
		{Text: "",
			UrlTemplate: ""},
	}
	links, err := makeResourceLinks("someNamespace", "someName", "someKind", linkTemplates)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(links))
}

func Test_makeLeftBarLinks_JustPassthrough(t *testing.T) {
	lbLinks := []LinkTemplate{
		{Text: "foo", UrlTemplate: "http://some-url.com"},
	}
	clinks, err := makeLeftBarLinks(lbLinks)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(clinks))
	assert.Equal(t, "foo", clinks[0].Text)
	assert.Equal(t, "http://some-url.com", clinks[0].Url)
}
