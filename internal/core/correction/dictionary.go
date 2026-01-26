package correction

import (
	"strings"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
)

func MergeDictionaries(global, request []config.DictionaryTerm) []config.DictionaryTerm {
	out := make([]config.DictionaryTerm, 0, len(global)+len(request))
	seen := make(map[string]int)

	add := func(term config.DictionaryTerm) {
		t := strings.TrimSpace(term.Term)
		if t == "" {
			return
		}

		aliases := make([]string, 0, len(term.Aliases))
		aliasSeen := make(map[string]struct{})
		for _, a := range term.Aliases {
			a = strings.TrimSpace(a)
			if a == "" {
				continue
			}
			if _, ok := aliasSeen[a]; ok {
				continue
			}
			aliasSeen[a] = struct{}{}
			aliases = append(aliases, a)
		}

		if idx, ok := seen[t]; ok {
			existing := out[idx]
			existingAliases := make(map[string]struct{}, len(existing.Aliases))
			for _, a := range existing.Aliases {
				existingAliases[a] = struct{}{}
			}
			for _, a := range aliases {
				if _, ok := existingAliases[a]; ok {
					continue
				}
				existing.Aliases = append(existing.Aliases, a)
			}
			out[idx] = existing
			return
		}

		seen[t] = len(out)
		out = append(out, config.DictionaryTerm{Term: t, Aliases: aliases})
	}

	for _, t := range global {
		add(t)
	}
	for _, t := range request {
		add(t)
	}

	return out
}
