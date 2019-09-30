/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * Licensed under the BSD 3-Clause license.
 * For full license text, see LICENSE.txt file in the repo root or
 * https://opensource.org/licenses/BSD-3-Clause
 */

package webserver

type LinkTemplate struct {
	Text        string `json:"text"`
	UrlTemplate string `json:"urlTemplate"`
}

type ResourceLinkTemplate struct {
	Text        string   `json:"text""`
	UrlTemplate string   `json:"urlTemplate"`
	Kinds       []string `json:"kinds"`
}

type ComputedLink struct {
	Text string
	Url  string
}

func makeResourceLinks(namespace string, name string, kind string, resLinks []ResourceLinkTemplate) ([]ComputedLink, error) {
	var ret []ComputedLink

	for _, thisLink := range resLinks {
		if thisLink.Text == "" {
			continue
		}
		data := struct {
			Name      string
			Namespace string
			Kind      string
		}{
			Name:      name,
			Namespace: namespace,
			Kind:      kind,
		}
		linkUrl, err := runTextTemplate(thisLink.UrlTemplate, data)
		if err != nil {
			return []ComputedLink{}, err
		}
		kindMatch := false
		if len(thisLink.Kinds) == 0 {
			kindMatch = true
		} else {
			for _, linkKind := range thisLink.Kinds {
				if linkKind == kind {
					kindMatch = true
				}
			}
		}

		if kindMatch {
			ret = append(ret, ComputedLink{Text: thisLink.Text, Url: linkUrl})
		}
	}
	return ret, nil
}

func makeLeftBarLinks(lbLinks []LinkTemplate) ([]ComputedLink, error) {
	var ret []ComputedLink

	for _, thisLink := range lbLinks {
		if thisLink.Text == "" {
			continue
		}
		// TODO: Add cluster
		data := struct{}{}
		linkUrl, err := runTextTemplate(thisLink.UrlTemplate, data)
		if err != nil {
			return []ComputedLink{}, err
		}

		ret = append(ret, ComputedLink{Text: thisLink.Text, Url: linkUrl})
	}
	return ret, nil
}
