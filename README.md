[![Release Workflow](https://github.com/Tensai75/nzb-monkey-go/actions/workflows/build_and_publish.yml/badge.svg?event=release)](https://github.com/Tensai75/nzb-monkey-go/actions/workflows/build_and_publish.yml)
[![Latest Release)](https://img.shields.io/github/v/release/Tensai75/nzb-monkey-go?logo=github)](https://github.com/Tensai75/nzb-monkey-go/releases/latest)

<img src="https://raw.githubusercontent.com/Tensai75/nzb-monkey-go/refs/heads/main/resources/nzb-monkey-go.svg" alt="Logo" width="128"/>

# NZB Monkey Go

A completely rewritten version in Golang of the [NZB Monkey](https://github.com/nzblnk/nzb-monkey), the reference implementation written in Python of how to handle a [NZBLNK](https://nzblnk.github.io/)-URI.
NZB Monkey Go also includes the functionality of the [NZBSearcher](https://github.com/Tensai75/nzbsearcher) so besides in the nzb search engines it can also search directly on the news server and create the NZB file from the article headers.

## Running the Monkey

Running the NZB Monkey Go is virtually identical to running the original NZB Monkey.

Differences are that in the configuration file, 'directsearch' can be enabled as an additional search engine in the section 'SEARCHENGINES' and an additional section 'DIRECTSEARCH' is included, with additional configuration options required for the direct search on the news server to work.

See the [NZB Monkey Go Wiki](https://github.com/Tensai75/nzb-monkey-go/wiki) for further information regarding installation and configuration.

Note that directsearch requires the NZBLNK to include the news groups (parameter 'g') and an additional parameter for the date ('d') when the post was uploaded to the Usenet.
Parameter 'd' can be provided either in the format 'DD.MM.YYYY' or as an unix timestamp.

![Monkey-Gif](https://github.com/Tensai75/nzb-monkey-go/raw/main/resources/nzbmonkey-go.gif)

## Metadata-assisted search for obfuscated posts

Some indexers show a friendly release title even when the Usenet article
subjects are randomized. In that case the regular `--subject` search cannot
find the upload. If you know the **original NNTP poster**, posted date,
newsgroup, **NZB file count**, and **total size**, you can explicitly request
a metadata-assisted direct search:

```sh
nzb-monkey-go \
  --subject "Example.Release.Name" \
  --group "a.b.example" \
  --date 1772172628 \
  --poster "someposter@example.invalid" \
  --expected-files 25 \
  --expected-size-gb 24.81 \
  --debug
```

`--date` accepts the normal `DD.MM.YYYY` or 10-digit Unix timestamp
formats. For an exact time use a **Unix timestamp** (check whether the
indexer displays local time or UTC). `--expected-size-gb` is **decimal GB**
(1 GB = 1,000,000,000 bytes). The normal `[DIRECTSEARCH]` NNTP server
credentials, date-window and retention requirements still apply. Enable
`directsearch` under `[SEARCHENGINES]`.

Supplying **all three** metadata flags switches directsearch to an article
overview scan. It filters by NNTP `From`, groups multipart subjects,
reconstructs an NZB from Message-IDs, and requires exactly one complete
candidate matching the requested file count and size within 15%. It **never
guesses** between multiple plausible results. Up to four NNTP connections
are used, with a cap on poster-matching headers to bound memory.

The mode prints the number of scanned headers, poster matches, parsed
articles, and the closest candidate groups. If it finds no result,
check the poster value (which may differ from the indexer's display name),
timezone, NNTP provider retention/overview access, and whether the archive
parts actually have a shared header/stem. Unrelated randomized filenames
cannot always be grouped safely from metadata alone. No article bodies are
downloaded during searching.

## Windows and Linux binaries

The binaries are available on the [release page](https://github.com/Tensai75/nzb-monkey-go/releases).

For Arch Linux / Manjao user a AUR package is kindly provided by [nzb-tuxxx](https://github.com/nzb-tuxxx): https://aur.archlinux.org/packages/nzb-monkey-go-bin

### macOS Support

Please see the beta app bundle release here: [NZB Monkey Go v0.4.0-beta1 (darwin only)](https://github.com/Tensai75/nzb-monkey-go/releases/tag/v0.4.0-beta1).
Feedback and error reports are welcome!

## Contribution

Feel free to send pull requests.
