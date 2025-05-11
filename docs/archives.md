You are a senior software architect. You produce optimized, maintainable code that follows best practices. 

Your task is to review the current codebase and suggest improvements or optimizations.

Rules:
- Keep your suggestions concise and focused. Avoid unnecessary explanations or fluff. 
- Your output should be a series of specific, actionable changes.

When approaching this task:
1. Carefully review the provided code.
2. Identify areas that could be improved in terms of efficiency, readability, or maintainability.
3. Consider best practices for the specific programming language used.
4. Think about potential optimizations that could enhance performance.
5. Look for opportunities to refactor or restructure the code for better organization.

For each suggested change, provide:
1. A short description of the change (one line maximum).
2. The modified code block.

Use the following format for your output:

[Short Description]
```[language]:[path/to/file]
[code block]
```

Begin your analysis and provide your suggestions now.

My current codebase:
<current_codebase>
Project Structure:
├── 7z.go
├── LICENSE
├── README.md
├── archives.go
├── archives_test.go
├── brotli.go
├── brotli_test.go
├── bz2.go
├── formats.go
├── formats_test.go
├── fs.go
├── fs_test.go
├── go.mod
├── go.sum
├── gz.go
├── interfaces.go
├── lz4.go
├── lzip.go
├── minlz.go
├── rar.go
├── rar_test.go
├── sz.go
├── tar.go
├── testdata
│   ├── self-tar.tar
│   ├── test.part01.rar
│   ├── test.part02.rar
│   ├── test.zip
│   └── unordered.zip
├── xz.go
├── zip.go
├── zlib.go
└── zstd.go


7z.go
```
1 | package archives
2 | 
3 | import (
4 | 	"bytes"
5 | 	"context"
6 | 	"errors"
7 | 	"fmt"
8 | 	"io"
9 | 	"io/fs"
10 | 	"log"
11 | 	"strings"
12 | 
13 | 	"github.com/bodgit/sevenzip"
14 | )
15 | 
16 | func init() {
17 | 	RegisterFormat(SevenZip{})
18 | 
19 | 	// looks like the sevenzip package registers a lot of decompressors for us automatically:
20 | 	// https://github.com/bodgit/sevenzip/blob/46c5197162c784318b98b9a3f80289a9aa1ca51a/register.go#L38-L61
21 | }
22 | 
23 | type SevenZip struct {
24 | 	// If true, errors encountered during reading or writing
25 | 	// a file within an archive will be logged and the
26 | 	// operation will continue on remaining files.
27 | 	ContinueOnError bool
28 | 
29 | 	// The password, if dealing with an encrypted archive.
30 | 	Password string
31 | }
32 | 
33 | func (SevenZip) Extension() string { return ".7z" }
34 | func (SevenZip) MediaType() string { return "application/x-7z-compressed" }
35 | 
36 | func (z SevenZip) Match(_ context.Context, filename string, stream io.Reader) (MatchResult, error) {
37 | 	var mr MatchResult
38 | 
39 | 	// match filename
40 | 	if strings.Contains(strings.ToLower(filename), z.Extension()) {
41 | 		mr.ByName = true
42 | 	}
43 | 
44 | 	// match file header
45 | 	buf, err := readAtMost(stream, len(sevenZipHeader))
46 | 	if err != nil {
47 | 		return mr, err
48 | 	}
49 | 	mr.ByStream = bytes.Equal(buf, sevenZipHeader)
50 | 
51 | 	return mr, nil
52 | }
53 | 
54 | // Archive is not implemented for 7z because I do not know of a pure-Go 7z writer.
55 | 
56 | // Extract extracts files from z, implementing the Extractor interface. Uniquely, however,
57 | // sourceArchive must be an io.ReaderAt and io.Seeker, which are oddly disjoint interfaces
58 | // from io.Reader which is what the method signature requires. We chose this signature for
59 | // the interface because we figure you can Read() from anything you can ReadAt() or Seek()
60 | // with. Due to the nature of the zip archive format, if sourceArchive is not an io.Seeker
61 | // and io.ReaderAt, an error is returned.
62 | func (z SevenZip) Extract(ctx context.Context, sourceArchive io.Reader, handleFile FileHandler) error {
63 | 	sra, ok := sourceArchive.(seekReaderAt)
64 | 	if !ok {
65 | 		return fmt.Errorf("input type must be an io.ReaderAt and io.Seeker because of zip format constraints")
66 | 	}
67 | 
68 | 	size, err := streamSizeBySeeking(sra)
69 | 	if err != nil {
70 | 		return fmt.Errorf("determining stream size: %w", err)
71 | 	}
72 | 
73 | 	zr, err := sevenzip.NewReaderWithPassword(sra, size, z.Password)
74 | 	if err != nil {
75 | 		return err
76 | 	}
77 | 
78 | 	// important to initialize to non-nil, empty value due to how fileIsIncluded works
79 | 	skipDirs := skipList{}
80 | 
81 | 	for i, f := range zr.File {
82 | 		if err := ctx.Err(); err != nil {
83 | 			return err // honor context cancellation
84 | 		}
85 | 
86 | 		if fileIsIncluded(skipDirs, f.Name) {
87 | 			continue
88 | 		}
89 | 
90 | 		fi := f.FileInfo()
91 | 		file := FileInfo{
92 | 			FileInfo:      fi,
93 | 			Header:        f.FileHeader,
94 | 			NameInArchive: f.Name,
95 | 			Open: func() (fs.File, error) {
96 | 				openedFile, err := f.Open()
97 | 				if err != nil {
98 | 					return nil, err
99 | 				}
100 | 				return fileInArchive{openedFile, fi}, nil
101 | 			},
102 | 		}
103 | 
104 | 		err := handleFile(ctx, file)
105 | 		if errors.Is(err, fs.SkipAll) {
106 | 			break
107 | 		} else if errors.Is(err, fs.SkipDir) && file.IsDir() {
108 | 			skipDirs.add(f.Name)
109 | 		} else if err != nil {
110 | 			if z.ContinueOnError {
111 | 				log.Printf("[ERROR] %s: %v", f.Name, err)
112 | 				continue
113 | 			}
114 | 			return fmt.Errorf("handling file %d: %s: %w", i, f.Name, err)
115 | 		}
116 | 	}
117 | 
118 | 	return nil
119 | }
120 | 
121 | // https://py7zr.readthedocs.io/en/latest/archive_format.html#signature
122 | var sevenZipHeader = []byte("7z\xBC\xAF\x27\x1C")
123 | 
124 | // Interface guard
125 | var _ Extractor = SevenZip{}
```

archives.go
```
1 | package archives
2 | 
3 | import (
4 | 	"context"
5 | 	"fmt"
6 | 	"io"
7 | 	"io/fs"
8 | 	"os"
9 | 	"path"
10 | 	"path/filepath"
11 | 	"strings"
12 | 	"time"
13 | )
14 | 
15 | // FileInfo is a virtualized, generalized file abstraction for interacting with archives.
16 | type FileInfo struct {
17 | 	fs.FileInfo
18 | 
19 | 	// The file header as used/provided by the archive format.
20 | 	// Typically, you do not need to set this field when creating
21 | 	// an archive.
22 | 	Header any
23 | 
24 | 	// The path of the file as it appears in the archive.
25 | 	// This is equivalent to Header.Name (for most Header
26 | 	// types). We require it to be specified here because
27 | 	// it is such a common field and we want to preserve
28 | 	// format-agnosticism (no type assertions) for basic
29 | 	// operations.
30 | 	//
31 | 	// When extracting, this name or path may not have
32 | 	// been sanitized; it should not be trusted at face
33 | 	// value. Consider using path.Clean() before using.
34 | 	//
35 | 	// If this is blank when inserting a file into an
36 | 	// archive, the filename's base may be assumed
37 | 	// by default to be the name in the archive.
38 | 	NameInArchive string
39 | 
40 | 	// For symbolic and hard links, the target of the link.
41 | 	// Not supported by all archive formats.
42 | 	LinkTarget string
43 | 
44 | 	// A callback function that opens the file to read its
45 | 	// contents. The file must be closed when reading is
46 | 	// complete.
47 | 	Open func() (fs.File, error)
48 | }
49 | 
50 | func (f FileInfo) Stat() (fs.FileInfo, error) { return f.FileInfo, nil }
51 | 
52 | // FilesFromDisk is an opinionated function that returns a list of FileInfos
53 | // by walking the directories in the filenames map. The keys are the names on
54 | // disk, and the values become their associated names in the archive.
55 | //
56 | // Map keys that specify directories on disk will be walked and added to the
57 | // archive recursively, rooted at the named directory. They should use the
58 | // platform's path separator (backslash on Windows; slash on everything else).
59 | // For convenience, map keys that end in a separator ('/', or '\' on Windows)
60 | // will enumerate contents only, without adding the folder itself to the archive.
61 | //
62 | // Map values should typically use slash ('/') as the separator regardless of
63 | // the platform, as most archive formats standardize on that rune as the
64 | // directory separator for filenames within an archive. For convenience, map
65 | // values that are empty string are interpreted as the base name of the file
66 | // (sans path) in the root of the archive; and map values that end in a slash
67 | // will use the base name of the file in that folder of the archive.
68 | //
69 | // File gathering will adhere to the settings specified in options.
70 | //
71 | // This function is used primarily when preparing a list of files to add to
72 | // an archive.
73 | func FilesFromDisk(ctx context.Context, options *FromDiskOptions, filenames map[string]string) ([]FileInfo, error) {
74 | 	var files []FileInfo
75 | 	for rootOnDisk, rootInArchive := range filenames {
76 | 		if err := ctx.Err(); err != nil {
77 | 			return nil, err
78 | 		}
79 | 
80 | 		walkErr := filepath.WalkDir(rootOnDisk, func(filename string, d fs.DirEntry, err error) error {
81 | 			if err := ctx.Err(); err != nil {
82 | 				return err
83 | 			}
84 | 			if err != nil {
85 | 				return err
86 | 			}
87 | 
88 | 			info, err := d.Info()
89 | 			if err != nil {
90 | 				return err
91 | 			}
92 | 
93 | 			nameInArchive := nameOnDiskToNameInArchive(filename, rootOnDisk, rootInArchive)
94 | 			// this is the root folder and we are adding its contents to target rootInArchive
95 | 			if info.IsDir() && nameInArchive == "" {
96 | 				return nil
97 | 			}
98 | 
99 | 			// handle symbolic links
100 | 			var linkTarget string
101 | 			if isSymlink(info) {
102 | 				if options != nil && options.FollowSymlinks {
103 | 					// dereference symlinks
104 | 					filename, err = os.Readlink(filename)
105 | 					if err != nil {
106 | 						return fmt.Errorf("%s: readlink: %w", filename, err)
107 | 					}
108 | 					info, err = os.Stat(filename)
109 | 					if err != nil {
110 | 						return fmt.Errorf("%s: statting dereferenced symlink: %w", filename, err)
111 | 					}
112 | 				} else {
113 | 					// preserve symlinks
114 | 					linkTarget, err = os.Readlink(filename)
115 | 					if err != nil {
116 | 						return fmt.Errorf("%s: readlink: %w", filename, err)
117 | 					}
118 | 				}
119 | 			}
120 | 
121 | 			// handle file attributes
122 | 			if options != nil && options.ClearAttributes {
123 | 				info = noAttrFileInfo{info}
124 | 			}
125 | 
126 | 			file := FileInfo{
127 | 				FileInfo:      info,
128 | 				NameInArchive: nameInArchive,
129 | 				LinkTarget:    linkTarget,
130 | 				Open: func() (fs.File, error) {
131 | 					return os.Open(filename)
132 | 				},
133 | 			}
134 | 
135 | 			files = append(files, file)
136 | 			return nil
137 | 		})
138 | 		if walkErr != nil {
139 | 			return nil, walkErr
140 | 		}
141 | 	}
142 | 	return files, nil
143 | }
144 | 
145 | // nameOnDiskToNameInArchive converts a filename from disk to a name in an archive,
146 | // respecting rules defined by FilesFromDisk. nameOnDisk is the full filename on disk
147 | // which is expected to be prefixed by rootOnDisk (according to fs.WalkDirFunc godoc)
148 | // and which will be placed into a folder rootInArchive in the archive.
149 | func nameOnDiskToNameInArchive(nameOnDisk, rootOnDisk, rootInArchive string) string {
150 | 	// These manipulations of rootInArchive could be done just once instead of on
151 | 	// every walked file since they don't rely on nameOnDisk which is the only
152 | 	// variable that changes during the walk, but combining all the logic into this
153 | 	// one function is easier to reason about and test. I suspect the performance
154 | 	// penalty is insignificant.
155 | 	if strings.HasSuffix(rootOnDisk, string(filepath.Separator)) {
156 | 		// "map keys that end in a separator will enumerate contents only,
157 | 		// without adding the folder itself to the archive."
158 | 		rootInArchive = trimTopDir(rootInArchive)
159 | 	} else if rootInArchive == "" {
160 | 		// "map values that are empty string are interpreted as the base name
161 | 		// of the file (sans path) in the root of the archive"
162 | 		rootInArchive = filepath.Base(rootOnDisk)
163 | 	}
164 | 	if rootInArchive == "." {
165 | 		// an in-archive root of "." is an escape hatch for the above rule
166 | 		// where an empty in-archive root means to use the base name of the
167 | 		// file; if the user does not want this, they can specify a "." to
168 | 		// still put it in the root of the archive
169 | 		rootInArchive = ""
170 | 	}
171 | 	if strings.HasSuffix(rootInArchive, "/") {
172 | 		// "map values that end in a slash will use the base name of the file in
173 | 		// that folder of the archive."
174 | 		rootInArchive += filepath.Base(rootOnDisk)
175 | 	}
176 | 	truncPath := strings.TrimPrefix(nameOnDisk, rootOnDisk)
177 | 	return path.Join(rootInArchive, filepath.ToSlash(truncPath))
178 | }
179 | 
180 | // trimTopDir strips the top or first directory from the path.
181 | // It expects a forward-slashed path.
182 | //
183 | // Examples: "a/b/c" => "b/c", "/a/b/c" => "b/c"
184 | func trimTopDir(dir string) string {
185 | 	return strings.TrimPrefix(dir, topDir(dir)+"/")
186 | }
187 | 
188 | // topDir returns the top or first directory in the path.
189 | // It expects a forward-slashed path.
190 | //
191 | // Examples: "a/b/c" => "a", "/a/b/c" => "/a"
192 | func topDir(dir string) string {
193 | 	var start int
194 | 	if len(dir) > 0 && dir[0] == '/' {
195 | 		start = 1
196 | 	}
197 | 	if pos := strings.Index(dir[start:], "/"); pos >= 0 {
198 | 		return dir[:pos+start]
199 | 	}
200 | 	return dir
201 | }
202 | 
203 | // noAttrFileInfo is used to zero out some file attributes (issue #280).
204 | type noAttrFileInfo struct{ fs.FileInfo }
205 | 
206 | // Mode preserves only the type and permission bits.
207 | func (no noAttrFileInfo) Mode() fs.FileMode {
208 | 	return no.FileInfo.Mode() & (fs.ModeType | fs.ModePerm)
209 | }
210 | func (noAttrFileInfo) ModTime() time.Time { return time.Time{} }
211 | func (noAttrFileInfo) Sys() any           { return nil }
212 | 
213 | // FromDiskOptions specifies various options for gathering files from disk.
214 | type FromDiskOptions struct {
215 | 	// If true, symbolic links will be dereferenced, meaning that
216 | 	// the link will not be added as a link, but what the link
217 | 	// points to will be added as a file.
218 | 	FollowSymlinks bool
219 | 
220 | 	// If true, some file attributes will not be preserved.
221 | 	// Name, size, type, and permissions will still be preserved.
222 | 	ClearAttributes bool
223 | }
224 | 
225 | // FileHandler is a callback function that is used to handle files as they are read
226 | // from an archive; it is kind of like fs.WalkDirFunc. Handler functions that open
227 | // their files must not overlap or run concurrently, as files may be read from the
228 | // same sequential stream; always close the file before returning.
229 | //
230 | // If the special error value fs.SkipDir is returned, the directory of the file
231 | // (or the file itself if it is a directory) will not be walked. Note that because
232 | // archive contents are not necessarily ordered, skipping directories requires
233 | // memory, and skipping lots of directories may run up your memory bill.
234 | //
235 | // Any other returned error will terminate a walk and be returned to the caller.
236 | type FileHandler func(ctx context.Context, info FileInfo) error
237 | 
238 | // openAndCopyFile opens file for reading, copies its
239 | // contents to w, then closes file.
240 | func openAndCopyFile(file FileInfo, w io.Writer) error {
241 | 	fileReader, err := file.Open()
242 | 	if err != nil {
243 | 		return err
244 | 	}
245 | 	defer fileReader.Close()
246 | 	// When file is in use and size is being written to, creating the compressed
247 | 	// file will fail with "archive/tar: write too long." Using CopyN gracefully
248 | 	// handles this.
249 | 	_, err = io.Copy(w, fileReader)
250 | 	if err != nil && err != io.EOF {
251 | 		return err
252 | 	}
253 | 	return nil
254 | }
255 | 
256 | // fileIsIncluded returns true if filename is included according to
257 | // filenameList; meaning it is in the list, its parent folder/path
258 | // is in the list, or the list is nil.
259 | func fileIsIncluded(filenameList []string, filename string) bool {
260 | 	// include all files if there is no specific list
261 | 	if filenameList == nil {
262 | 		return true
263 | 	}
264 | 	for _, fn := range filenameList {
265 | 		// exact matches are of course included
266 | 		if filename == fn {
267 | 			return true
268 | 		}
269 | 		// also consider the file included if its parent folder/path is in the list
270 | 		if strings.HasPrefix(filename, strings.TrimSuffix(fn, "/")+"/") {
271 | 			return true
272 | 		}
273 | 	}
274 | 	return false
275 | }
276 | 
277 | func isSymlink(info fs.FileInfo) bool {
278 | 	return info.Mode()&os.ModeSymlink != 0
279 | }
280 | 
281 | // streamSizeBySeeking determines the size of the stream by
282 | // seeking to the end, then back again, so the resulting
283 | // seek position upon returning is the same as when called
284 | // (assuming no errors).
285 | func streamSizeBySeeking(s io.Seeker) (int64, error) {
286 | 	currentPosition, err := s.Seek(0, io.SeekCurrent)
287 | 	if err != nil {
288 | 		return 0, fmt.Errorf("getting current offset: %w", err)
289 | 	}
290 | 	maxPosition, err := s.Seek(0, io.SeekEnd)
291 | 	if err != nil {
292 | 		return 0, fmt.Errorf("fast-forwarding to end: %w", err)
293 | 	}
294 | 	_, err = s.Seek(currentPosition, io.SeekStart)
295 | 	if err != nil {
296 | 		return 0, fmt.Errorf("returning to prior offset %d: %w", currentPosition, err)
297 | 	}
298 | 	return maxPosition, nil
299 | }
300 | 
301 | // skipList keeps a list of non-intersecting paths
302 | // as long as its add method is used. Identical
303 | // elements are rejected, more specific paths are
304 | // replaced with broader ones, and more specific
305 | // paths won't be added when a broader one already
306 | // exists in the list. Trailing slashes are ignored.
307 | type skipList []string
308 | 
309 | func (s *skipList) add(dir string) {
310 | 	trimmedDir := strings.TrimSuffix(dir, "/")
311 | 	var dontAdd bool
312 | 	for i := 0; i < len(*s); i++ {
313 | 		trimmedElem := strings.TrimSuffix((*s)[i], "/")
314 | 		if trimmedDir == trimmedElem {
315 | 			return
316 | 		}
317 | 		// don't add dir if a broader path already exists in the list
318 | 		if strings.HasPrefix(trimmedDir, trimmedElem+"/") {
319 | 			dontAdd = true
320 | 			continue
321 | 		}
322 | 		// if dir is broader than a path in the list, remove more specific path in list
323 | 		if strings.HasPrefix(trimmedElem, trimmedDir+"/") {
324 | 			*s = append((*s)[:i], (*s)[i+1:]...)
325 | 			i--
326 | 		}
327 | 	}
328 | 	if !dontAdd {
329 | 		*s = append(*s, dir)
330 | 	}
331 | }
```

archives_test.go
```
1 | package archives
2 | 
3 | import (
4 | 	"reflect"
5 | 	"runtime"
6 | 	"strings"
7 | 	"testing"
8 | )
9 | 
10 | func TestTrimTopDir(t *testing.T) {
11 | 	for i, test := range []struct {
12 | 		input string
13 | 		want  string
14 | 	}{
15 | 		{input: "a/b/c", want: "b/c"},
16 | 		{input: "a", want: "a"},
17 | 		{input: "abc/def", want: "def"},
18 | 		{input: "/abc/def", want: "def"},
19 | 	} {
20 | 		t.Run(test.input, func(t *testing.T) {
21 | 			got := trimTopDir(test.input)
22 | 			if got != test.want {
23 | 				t.Errorf("Test %d: want: '%s', got: '%s')", i, test.want, got)
24 | 			}
25 | 		})
26 | 	}
27 | }
28 | 
29 | func TestTopDir(t *testing.T) {
30 | 	for _, tc := range []struct {
31 | 		input string
32 | 		want  string
33 | 	}{
34 | 		{input: "a/b/c", want: "a"},
35 | 		{input: "a", want: "a"},
36 | 		{input: "abc/def", want: "abc"},
37 | 		{input: "/abc/def", want: "/abc"},
38 | 	} {
39 | 		t.Run(tc.input, func(t *testing.T) {
40 | 			got := topDir(tc.input)
41 | 			if got != tc.want {
42 | 				t.Errorf("want: '%s', got: '%s')", tc.want, got)
43 | 			}
44 | 		})
45 | 	}
46 | }
47 | 
48 | func TestFileIsIncluded(t *testing.T) {
49 | 	for i, tc := range []struct {
50 | 		included  []string
51 | 		candidate string
52 | 		expect    bool
53 | 	}{
54 | 		{
55 | 			included:  []string{"a"},
56 | 			candidate: "a",
57 | 			expect:    true,
58 | 		},
59 | 		{
60 | 			included:  []string{"a", "b", "a/b"},
61 | 			candidate: "b",
62 | 			expect:    true,
63 | 		},
64 | 		{
65 | 			included:  []string{"a", "b", "c/d"},
66 | 			candidate: "c/d/e",
67 | 			expect:    true,
68 | 		},
69 | 		{
70 | 			included:  []string{"a"},
71 | 			candidate: "a/b/c",
72 | 			expect:    true,
73 | 		},
74 | 		{
75 | 			included:  []string{"a"},
76 | 			candidate: "aa/b/c",
77 | 			expect:    false,
78 | 		},
79 | 		{
80 | 			included:  []string{"a", "b", "c/d"},
81 | 			candidate: "b/c",
82 | 			expect:    true,
83 | 		},
84 | 		{
85 | 			included:  []string{"a/"},
86 | 			candidate: "a",
87 | 			expect:    false,
88 | 		},
89 | 		{
90 | 			included:  []string{"a/"},
91 | 			candidate: "a/",
92 | 			expect:    true,
93 | 		},
94 | 		{
95 | 			included:  []string{"a"},
96 | 			candidate: "a/",
97 | 			expect:    true,
98 | 		},
99 | 		{
100 | 			included:  []string{"a/b"},
101 | 			candidate: "a/",
102 | 			expect:    false,
103 | 		},
104 | 	} {
105 | 		actual := fileIsIncluded(tc.included, tc.candidate)
106 | 		if actual != tc.expect {
107 | 			t.Errorf("Test %d (included=%v candidate=%v): expected %t but got %t",
108 | 				i, tc.included, tc.candidate, tc.expect, actual)
109 | 		}
110 | 	}
111 | }
112 | 
113 | func TestSkipList(t *testing.T) {
114 | 	for i, tc := range []struct {
115 | 		start  skipList
116 | 		add    string
117 | 		expect skipList
118 | 	}{
119 | 		{
120 | 			start:  skipList{"a", "b", "c"},
121 | 			add:    "d",
122 | 			expect: skipList{"a", "b", "c", "d"},
123 | 		},
124 | 		{
125 | 			start:  skipList{"a", "b", "c"},
126 | 			add:    "b",
127 | 			expect: skipList{"a", "b", "c"},
128 | 		},
129 | 		{
130 | 			start:  skipList{"a", "b", "c"},
131 | 			add:    "b/c", // don't add because b implies b/c
132 | 			expect: skipList{"a", "b", "c"},
133 | 		},
134 | 		{
135 | 			start:  skipList{"a", "b", "c"},
136 | 			add:    "b/c/", // effectively same as above
137 | 			expect: skipList{"a", "b", "c"},
138 | 		},
139 | 		{
140 | 			start:  skipList{"a", "b/", "c"},
141 | 			add:    "b", // effectively same as b/
142 | 			expect: skipList{"a", "b/", "c"},
143 | 		},
144 | 		{
145 | 			start:  skipList{"a", "b/c", "c"},
146 | 			add:    "b", // replace b/c because b is broader
147 | 			expect: skipList{"a", "c", "b"},
148 | 		},
149 | 	} {
150 | 		start := make(skipList, len(tc.start))
151 | 		copy(start, tc.start)
152 | 
153 | 		tc.start.add(tc.add)
154 | 
155 | 		if !reflect.DeepEqual(tc.start, tc.expect) {
156 | 			t.Errorf("Test %d (start=%v add=%v): expected %v but got %v",
157 | 				i, start, tc.add, tc.expect, tc.start)
158 | 		}
159 | 	}
160 | }
161 | 
162 | func TestNameOnDiskToNameInArchive(t *testing.T) {
163 | 	for i, tc := range []struct {
164 | 		windows       bool   // only run this test on Windows
165 | 		rootOnDisk    string // user says they want to archive this file/folder
166 | 		nameOnDisk    string // the walk encounters a file with this name (with rootOnDisk as a prefix)
167 | 		rootInArchive string // file should be placed in this dir within the archive (rootInArchive becomes a prefix)
168 | 		expect        string // final filename in archive
169 | 	}{
170 | 		{
171 | 			rootOnDisk:    "a",
172 | 			nameOnDisk:    "a/b/c",
173 | 			rootInArchive: "",
174 | 			expect:        "a/b/c",
175 | 		},
176 | 		{
177 | 			rootOnDisk:    "a/b",
178 | 			nameOnDisk:    "a/b/c",
179 | 			rootInArchive: "",
180 | 			expect:        "b/c",
181 | 		},
182 | 		{
183 | 			rootOnDisk:    "a/b/",
184 | 			nameOnDisk:    "a/b/c",
185 | 			rootInArchive: "",
186 | 			expect:        "c",
187 | 		},
188 | 		{
189 | 			rootOnDisk:    "a/b/",
190 | 			nameOnDisk:    "a/b/c",
191 | 			rootInArchive: ".",
192 | 			expect:        "c",
193 | 		},
194 | 		{
195 | 			rootOnDisk:    "a/b/c",
196 | 			nameOnDisk:    "a/b/c",
197 | 			rootInArchive: "",
198 | 			expect:        "c",
199 | 		},
200 | 		{
201 | 			rootOnDisk:    "a/b",
202 | 			nameOnDisk:    "a/b/c",
203 | 			rootInArchive: "foo",
204 | 			expect:        "foo/c",
205 | 		},
206 | 		{
207 | 			rootOnDisk:    "a",
208 | 			nameOnDisk:    "a/b/c",
209 | 			rootInArchive: "foo",
210 | 			expect:        "foo/b/c",
211 | 		},
212 | 		{
213 | 			rootOnDisk:    "a",
214 | 			nameOnDisk:    "a/b/c",
215 | 			rootInArchive: "foo/",
216 | 			expect:        "foo/a/b/c",
217 | 		},
218 | 		{
219 | 			rootOnDisk:    "a/",
220 | 			nameOnDisk:    "a/b/c",
221 | 			rootInArchive: "foo",
222 | 			expect:        "foo/b/c",
223 | 		},
224 | 		{
225 | 			rootOnDisk:    "a/",
226 | 			nameOnDisk:    "a/b/c",
227 | 			rootInArchive: "foo",
228 | 			expect:        "foo/b/c",
229 | 		},
230 | 		{
231 | 			windows:       true,
232 | 			rootOnDisk:    `C:\foo`,
233 | 			nameOnDisk:    `C:\foo\bar`,
234 | 			rootInArchive: "",
235 | 			expect:        "foo/bar",
236 | 		},
237 | 		{
238 | 			windows:       true,
239 | 			rootOnDisk:    `C:\foo`,
240 | 			nameOnDisk:    `C:\foo\bar`,
241 | 			rootInArchive: "subfolder",
242 | 			expect:        "subfolder/bar",
243 | 		},
244 | 	} {
245 | 		if !strings.HasPrefix(tc.nameOnDisk, tc.rootOnDisk) {
246 | 			t.Errorf("Test %d: Invalid test case! Filename (on disk) will have rootOnDisk as a prefix according to the fs.WalkDirFunc godoc.", i)
247 | 			continue
248 | 		}
249 | 		if tc.windows && runtime.GOOS != "windows" {
250 | 			t.Logf("Test %d: Skipping test that is only compatible with Windows", i)
251 | 			continue
252 | 		}
253 | 		if !tc.windows && runtime.GOOS == "windows" {
254 | 			t.Logf("Test %d: Skipping test that is not compatible with Windows", i)
255 | 			continue
256 | 		}
257 | 
258 | 		actual := nameOnDiskToNameInArchive(tc.nameOnDisk, tc.rootOnDisk, tc.rootInArchive)
259 | 		if actual != tc.expect {
260 | 			t.Errorf("Test %d: Got '%s' but expected '%s' (nameOnDisk=%s rootOnDisk=%s rootInArchive=%s)",
261 | 				i, actual, tc.expect, tc.nameOnDisk, tc.rootOnDisk, tc.rootInArchive)
262 | 		}
263 | 	}
264 | }
```

brotli.go
```
1 | package archives
2 | 
3 | import (
4 | 	"bytes"
5 | 	"context"
6 | 	"io"
7 | 	"strings"
8 | 
9 | 	"github.com/andybalholm/brotli"
10 | )
11 | 
12 | func init() {
13 | 	RegisterFormat(Brotli{})
14 | }
15 | 
16 | // Brotli facilitates brotli compression.
17 | type Brotli struct {
18 | 	Quality int
19 | }
20 | 
21 | func (Brotli) Extension() string { return ".br" }
22 | func (Brotli) MediaType() string { return "application/x-br" }
23 | 
24 | func (br Brotli) Match(_ context.Context, filename string, stream io.Reader) (MatchResult, error) {
25 | 	var mr MatchResult
26 | 
27 | 	// match filename
28 | 	if strings.Contains(strings.ToLower(filename), br.Extension()) {
29 | 		mr.ByName = true
30 | 	}
31 | 
32 | 	if stream != nil {
33 | 		// brotli does not have well-defined file headers or a magic number;
34 | 		// the best way to match the stream is probably to try decoding part
35 | 		// of it, but we'll just have to guess a large-enough size that is
36 | 		// still small enough for the smallest streams we'll encounter
37 | 		input := &bytes.Buffer{}
38 | 		r := brotli.NewReader(io.TeeReader(stream, input))
39 | 		buf := make([]byte, 16)
40 | 
41 | 		// First gauntlet - can the reader even read 16 bytes without an error?
42 | 		n, err := r.Read(buf)
43 | 		if err != nil {
44 | 			return mr, nil
45 | 		}
46 | 		buf = buf[:n]
47 | 		inputBytes := input.Bytes()
48 | 
49 | 		// Second gauntlet - do the decompressed bytes exist in the raw input?
50 | 		// If they don't appear in the first 4 bytes (to account for the up to
51 | 		// 32 bits of initial brotli header) or at all, then chances are the
52 | 		// input was compressed.
53 | 		idx := bytes.Index(inputBytes, buf)
54 | 		if idx < 4 {
55 | 			mr.ByStream = true
56 | 			return mr, nil
57 | 		}
58 | 
59 | 		// The input is assumed to be compressed data, but we still can't be 100% sure.
60 | 		// Try reading more data until we encounter an error.
61 | 		for n < 128 {
62 | 			nn, err := r.Read(buf)
63 | 			switch err {
64 | 			case io.EOF:
65 | 				// If we've reached EOF, we return assuming it's compressed.
66 | 				mr.ByStream = true
67 | 				return mr, nil
68 | 			case io.ErrUnexpectedEOF:
69 | 				// If we've encountered a short read, that's probably due to invalid reads due
70 | 				// to the fact it isn't compressed data at all.
71 | 				return mr, nil
72 | 			case nil:
73 | 				// No error, no problem. Continue reading.
74 | 				n += nn
75 | 			default:
76 | 				// If we encounter any other error, return it.
77 | 				return mr, nil
78 | 			}
79 | 		}
80 | 
81 | 		// If we haven't encountered an error by now, the input is probably compressed.
82 | 		mr.ByStream = true
83 | 	}
84 | 
85 | 	return mr, nil
86 | }
87 | 
88 | func (br Brotli) OpenWriter(w io.Writer) (io.WriteCloser, error) {
89 | 	return brotli.NewWriterLevel(w, br.Quality), nil
90 | }
91 | 
92 | func (Brotli) OpenReader(r io.Reader) (io.ReadCloser, error) {
93 | 	return io.NopCloser(brotli.NewReader(r)), nil
94 | }
```

brotli_test.go
```
1 | package archives
2 | 
3 | import (
4 | 	"bytes"
5 | 	"context"
6 | 	"testing"
7 | )
8 | 
9 | func TestBrotli_Match_Stream(t *testing.T) {
10 | 	testTxt := []byte("this is text, but it has to be long enough to match brotli which doesn't have a magic number")
11 | 	type testcase struct {
12 | 		name    string
13 | 		input   []byte
14 | 		matches bool
15 | 	}
16 | 	for _, tc := range []testcase{
17 | 		{
18 | 			name:    "uncompressed yaml",
19 | 			input:   []byte("---\nthis-is-not-brotli: \"it is actually yaml\""),
20 | 			matches: false,
21 | 		},
22 | 		{
23 | 			name:    "uncompressed text",
24 | 			input:   testTxt,
25 | 			matches: false,
26 | 		},
27 | 		{
28 | 			name:    "text compressed with brotli quality 0",
29 | 			input:   compress(t, ".br", testTxt, Brotli{Quality: 0}.OpenWriter),
30 | 			matches: true,
31 | 		},
32 | 		{
33 | 			name:    "text compressed with brotli quality 1",
34 | 			input:   compress(t, ".br", testTxt, Brotli{Quality: 1}.OpenWriter),
35 | 			matches: true,
36 | 		},
37 | 		{
38 | 			name:    "text compressed with brotli quality 2",
39 | 			input:   compress(t, ".br", testTxt, Brotli{Quality: 2}.OpenWriter),
40 | 			matches: true,
41 | 		},
42 | 		{
43 | 			name:    "text compressed with brotli quality 3",
44 | 			input:   compress(t, ".br", testTxt, Brotli{Quality: 3}.OpenWriter),
45 | 			matches: true,
46 | 		},
47 | 		{
48 | 			name:    "text compressed with brotli quality 4",
49 | 			input:   compress(t, ".br", testTxt, Brotli{Quality: 4}.OpenWriter),
50 | 			matches: true,
51 | 		},
52 | 		{
53 | 			name:    "text compressed with brotli quality 5",
54 | 			input:   compress(t, ".br", testTxt, Brotli{Quality: 5}.OpenWriter),
55 | 			matches: true,
56 | 		},
57 | 		{
58 | 			name:    "text compressed with brotli quality 6",
59 | 			input:   compress(t, ".br", testTxt, Brotli{Quality: 6}.OpenWriter),
60 | 			matches: true,
61 | 		},
62 | 		{
63 | 			name:    "text compressed with brotli quality 7",
64 | 			input:   compress(t, ".br", testTxt, Brotli{Quality: 7}.OpenWriter),
65 | 			matches: true,
66 | 		},
67 | 		{
68 | 			name:    "text compressed with brotli quality 8",
69 | 			input:   compress(t, ".br", testTxt, Brotli{Quality: 8}.OpenWriter),
70 | 			matches: true,
71 | 		},
72 | 		{
73 | 			name:    "text compressed with brotli quality 9",
74 | 			input:   compress(t, ".br", testTxt, Brotli{Quality: 9}.OpenWriter),
75 | 			matches: true,
76 | 		},
77 | 		{
78 | 			name:    "text compressed with brotli quality 10",
79 | 			input:   compress(t, ".br", testTxt, Brotli{Quality: 10}.OpenWriter),
80 | 			matches: true,
81 | 		},
82 | 		{
83 | 			name:    "text compressed with brotli quality 11",
84 | 			input:   compress(t, ".br", testTxt, Brotli{Quality: 11}.OpenWriter),
85 | 			matches: true,
86 | 		},
87 | 	} {
88 | 		t.Run(tc.name, func(t *testing.T) {
89 | 			r := bytes.NewBuffer(tc.input)
90 | 
91 | 			mr, err := Brotli{}.Match(context.Background(), "", r)
92 | 			if err != nil {
93 | 				t.Errorf("Brotli.OpenReader() error = %v", err)
94 | 				return
95 | 			}
96 | 
97 | 			if mr.ByStream != tc.matches {
98 | 				t.Logf("input: %s", tc.input)
99 | 				t.Error("Brotli.Match() expected ByStream to be", tc.matches, "but got", mr.ByStream)
100 | 			}
101 | 		})
102 | 	}
103 | }
```

bz2.go
```
1 | package archives
2 | 
3 | import (
4 | 	"bytes"
5 | 	"context"
6 | 	"io"
7 | 	"strings"
8 | 
9 | 	"github.com/dsnet/compress/bzip2"
10 | )
11 | 
12 | func init() {
13 | 	RegisterFormat(Bz2{})
14 | }
15 | 
16 | // Bz2 facilitates bzip2 compression.
17 | type Bz2 struct {
18 | 	CompressionLevel int
19 | }
20 | 
21 | func (Bz2) Extension() string { return ".bz2" }
22 | func (Bz2) MediaType() string { return "application/x-bzip2" }
23 | 
24 | func (bz Bz2) Match(_ context.Context, filename string, stream io.Reader) (MatchResult, error) {
25 | 	var mr MatchResult
26 | 
27 | 	// match filename
28 | 	if strings.Contains(strings.ToLower(filename), bz.Extension()) {
29 | 		mr.ByName = true
30 | 	}
31 | 
32 | 	// match file header
33 | 	buf, err := readAtMost(stream, len(bzip2Header))
34 | 	if err != nil {
35 | 		return mr, err
36 | 	}
37 | 	mr.ByStream = bytes.Equal(buf, bzip2Header)
38 | 
39 | 	return mr, nil
40 | }
41 | 
42 | func (bz Bz2) OpenWriter(w io.Writer) (io.WriteCloser, error) {
43 | 	return bzip2.NewWriter(w, &bzip2.WriterConfig{
44 | 		Level: bz.CompressionLevel,
45 | 	})
46 | }
47 | 
48 | func (Bz2) OpenReader(r io.Reader) (io.ReadCloser, error) {
49 | 	return bzip2.NewReader(r, nil)
50 | }
51 | 
52 | var bzip2Header = []byte("BZh")
```

formats.go
```
1 | package archives
2 | 
3 | import (
4 | 	"bytes"
5 | 	"context"
6 | 	"errors"
7 | 	"fmt"
8 | 	"io"
9 | 	"path"
10 | 	"path/filepath"
11 | 	"strings"
12 | )
13 | 
14 | // RegisterFormat registers a format. It should be called during init.
15 | // Duplicate formats by name are not allowed and will panic.
16 | func RegisterFormat(format Format) {
17 | 	name := strings.Trim(strings.ToLower(format.Extension()), ".")
18 | 	if _, ok := formats[name]; ok {
19 | 		panic("format " + name + " is already registered")
20 | 	}
21 | 	formats[name] = format
22 | }
23 | 
24 | // Identify iterates the registered formats and returns the one that
25 | // matches the given filename and/or stream. It is capable of identifying
26 | // compressed files (.gz, .xz...), archive files (.tar, .zip...), and
27 | // compressed archive files (tar.gz, tar.bz2...). The returned Format
28 | // value can be type-asserted to ascertain its capabilities.
29 | //
30 | // If no matching formats were found, special error NoMatch is returned.
31 | //
32 | // If stream is nil then it will only match on file name and the
33 | // returned io.Reader will be nil.
34 | //
35 | // If stream is non-nil, it will be returned in the same read position
36 | // as it was before Identify() was called, by virtue of buffering the
37 | // peeked bytes. However, if the stream is an io.Seeker, Seek() must
38 | // work, no extra buffering will be performed, and the original input
39 | // value will be returned at the original position by seeking.
40 | func Identify(ctx context.Context, filename string, stream io.Reader) (Format, io.Reader, error) {
41 | 	var compression Compression
42 | 	var archival Archival
43 | 	var extraction Extraction
44 | 
45 | 	filename = path.Base(filepath.ToSlash(filename))
46 | 
47 | 	rewindableStream, err := newRewindReader(stream)
48 | 	if err != nil {
49 | 		return nil, nil, err
50 | 	}
51 | 
52 | 	// try compression format first, since that's the outer "layer" if combined
53 | 	for name, format := range formats {
54 | 		cf, isCompression := format.(Compression)
55 | 		if !isCompression {
56 | 			continue
57 | 		}
58 | 
59 | 		matchResult, err := identifyOne(ctx, format, filename, rewindableStream, nil)
60 | 		if err != nil {
61 | 			return nil, rewindableStream.reader(), fmt.Errorf("matching %s: %w", name, err)
62 | 		}
63 | 
64 | 		// if matched, wrap input stream with decompression
65 | 		// so we can see if it contains an archive within
66 | 		if matchResult.Matched() {
67 | 			compression = cf
68 | 			break
69 | 		}
70 | 	}
71 | 
72 | 	// try archival and extraction formats next
73 | 	for name, format := range formats {
74 | 		ar, isArchive := format.(Archival)
75 | 		ex, isExtract := format.(Extraction)
76 | 		if !isArchive && !isExtract {
77 | 			continue
78 | 		}
79 | 
80 | 		matchResult, err := identifyOne(ctx, format, filename, rewindableStream, compression)
81 | 		if err != nil {
82 | 			return nil, rewindableStream.reader(), fmt.Errorf("matching %s: %w", name, err)
83 | 		}
84 | 
85 | 		if matchResult.Matched() {
86 | 			archival = ar
87 | 			extraction = ex
88 | 			break
89 | 		}
90 | 	}
91 | 
92 | 	// the stream should be rewound by identifyOne; then return the most specific type of match
93 | 	bufferedStream := rewindableStream.reader()
94 | 	switch {
95 | 	case compression != nil && archival == nil && extraction == nil:
96 | 		return compression, bufferedStream, nil
97 | 	case compression == nil && archival != nil && extraction == nil:
98 | 		return archival, bufferedStream, nil
99 | 	case compression == nil && archival == nil && extraction != nil:
100 | 		return extraction, bufferedStream, nil
101 | 	case compression == nil && archival != nil && extraction != nil:
102 | 		// archival and extraction are always set together, so they must be the same
103 | 		return archival, bufferedStream, nil
104 | 	case compression != nil && extraction != nil:
105 | 		// in practice, this is only used for compressed tar files, and the tar format can
106 | 		// both read and write, so the archival value should always work too; but keep in
107 | 		// mind that Identify() is used on existing files to be read, not new files to write
108 | 		return CompressedArchive{archival, extraction, compression}, bufferedStream, nil
109 | 	default:
110 | 		return nil, bufferedStream, NoMatch
111 | 	}
112 | }
113 | 
114 | func identifyOne(ctx context.Context, format Format, filename string, stream *rewindReader, comp Compression) (mr MatchResult, err error) {
115 | 	defer stream.rewind()
116 | 
117 | 	if filename == "." {
118 | 		filename = ""
119 | 	}
120 | 
121 | 	// if looking within a compressed format, wrap the stream in a
122 | 	// reader that can decompress it so we can match the "inner" format
123 | 	// (yes, we have to make a new reader every time we do a match,
124 | 	// because we reset/seek the stream each time and that can mess up
125 | 	// the compression reader's state if we don't discard it also)
126 | 	if comp != nil && stream != nil {
127 | 		decompressedStream, openErr := comp.OpenReader(stream)
128 | 		if openErr != nil {
129 | 			return MatchResult{}, openErr
130 | 		}
131 | 		defer decompressedStream.Close()
132 | 		mr, err = format.Match(ctx, filename, decompressedStream)
133 | 	} else {
134 | 		// Make sure we pass a nil io.Reader not a *rewindReader(nil)
135 | 		var r io.Reader
136 | 		if stream != nil {
137 | 			r = stream
138 | 		}
139 | 		mr, err = format.Match(ctx, filename, r)
140 | 	}
141 | 
142 | 	// if the error is EOF, we can just ignore it.
143 | 	// Just means we have a small input file.
144 | 	if errors.Is(err, io.EOF) {
145 | 		err = nil
146 | 	}
147 | 	return mr, err
148 | }
149 | 
150 | // readAtMost reads at most n bytes from the stream. A nil, empty, or short
151 | // stream is not an error. The returned slice of bytes may have length < n
152 | // without an error.
153 | func readAtMost(stream io.Reader, n int) ([]byte, error) {
154 | 	if stream == nil || n <= 0 {
155 | 		return []byte{}, nil
156 | 	}
157 | 
158 | 	buf := make([]byte, n)
159 | 	nr, err := io.ReadFull(stream, buf)
160 | 
161 | 	// Return the bytes read if there was no error OR if the
162 | 	// error was EOF (stream was empty) or UnexpectedEOF (stream
163 | 	// had less than n). We ignore those errors because we aren't
164 | 	// required to read the full n bytes; so an empty or short
165 | 	// stream is not actually an error.
166 | 	if err == nil ||
167 | 		errors.Is(err, io.EOF) ||
168 | 		errors.Is(err, io.ErrUnexpectedEOF) {
169 | 		return buf[:nr], nil
170 | 	}
171 | 
172 | 	return nil, err
173 | }
174 | 
175 | // CompressedArchive represents an archive which is compressed externally
176 | // (for example, a gzipped tar file, .tar.gz.) It combines a compression
177 | // format on top of an archival/extraction format and provides both
178 | // functionalities in a single type, allowing archival and extraction
179 | // operations transparently through compression and decompression. However,
180 | // compressed archives have some limitations; for example, files cannot be
181 | // inserted/appended because of complexities with modifying existing
182 | // compression state (perhaps this could be overcome, but I'm not about to
183 | // try it).
184 | type CompressedArchive struct {
185 | 	Archival
186 | 	Extraction
187 | 	Compression
188 | }
189 | 
190 | // Name returns a concatenation of the archive and compression format extensions.
191 | func (ca CompressedArchive) Extension() string {
192 | 	var name string
193 | 	if ca.Archival != nil {
194 | 		name += ca.Archival.Extension()
195 | 	} else if ca.Extraction != nil {
196 | 		name += ca.Extraction.Extension()
197 | 	}
198 | 	name += ca.Compression.Extension()
199 | 	return name
200 | }
201 | 
202 | // MediaType returns the compression format's MIME type, since
203 | // a compressed archive is fundamentally a compressed file.
204 | func (ca CompressedArchive) MediaType() string { return ca.Compression.MediaType() }
205 | 
206 | // Match matches if the input matches both the compression and archival/extraction format.
207 | func (ca CompressedArchive) Match(ctx context.Context, filename string, stream io.Reader) (MatchResult, error) {
208 | 	var conglomerate MatchResult
209 | 
210 | 	if ca.Compression != nil {
211 | 		matchResult, err := ca.Compression.Match(ctx, filename, stream)
212 | 		if err != nil {
213 | 			return MatchResult{}, err
214 | 		}
215 | 		if !matchResult.Matched() {
216 | 			return matchResult, nil
217 | 		}
218 | 
219 | 		// wrap the reader with the decompressor so we can
220 | 		// attempt to match the archive by reading the stream
221 | 		rc, err := ca.Compression.OpenReader(stream)
222 | 		if err != nil {
223 | 			return matchResult, err
224 | 		}
225 | 		defer rc.Close()
226 | 		stream = rc
227 | 
228 | 		conglomerate = matchResult
229 | 	}
230 | 
231 | 	if ca.Archival != nil {
232 | 		matchResult, err := ca.Archival.Match(ctx, filename, stream)
233 | 		if err != nil {
234 | 			return MatchResult{}, err
235 | 		}
236 | 		if !matchResult.Matched() {
237 | 			return matchResult, nil
238 | 		}
239 | 		conglomerate.ByName = conglomerate.ByName || matchResult.ByName
240 | 		conglomerate.ByStream = conglomerate.ByStream || matchResult.ByStream
241 | 	}
242 | 
243 | 	return conglomerate, nil
244 | }
245 | 
246 | // Archive writes an archive to the output stream while compressing the result.
247 | func (ca CompressedArchive) Archive(ctx context.Context, output io.Writer, files []FileInfo) error {
248 | 	if ca.Archival == nil {
249 | 		return fmt.Errorf("no archival format")
250 | 	}
251 | 	if ca.Compression != nil {
252 | 		wc, err := ca.Compression.OpenWriter(output)
253 | 		if err != nil {
254 | 			return err
255 | 		}
256 | 		defer wc.Close()
257 | 		output = wc
258 | 	}
259 | 	return ca.Archival.Archive(ctx, output, files)
260 | }
261 | 
262 | // ArchiveAsync adds files to the output archive while compressing the result asynchronously.
263 | func (ca CompressedArchive) ArchiveAsync(ctx context.Context, output io.Writer, jobs <-chan ArchiveAsyncJob) error {
264 | 	if ca.Archival == nil {
265 | 		return fmt.Errorf("no archival format")
266 | 	}
267 | 	do, ok := ca.Archival.(ArchiverAsync)
268 | 	if !ok {
269 | 		return fmt.Errorf("%T archive does not support async writing", ca.Archival)
270 | 	}
271 | 	if ca.Compression != nil {
272 | 		wc, err := ca.Compression.OpenWriter(output)
273 | 		if err != nil {
274 | 			return err
275 | 		}
276 | 		defer wc.Close()
277 | 		output = wc
278 | 	}
279 | 	return do.ArchiveAsync(ctx, output, jobs)
280 | }
281 | 
282 | // Extract reads files out of a compressed archive while decompressing the results.
283 | func (ca CompressedArchive) Extract(ctx context.Context, sourceArchive io.Reader, handleFile FileHandler) error {
284 | 	if ca.Extraction == nil {
285 | 		return fmt.Errorf("no extraction format")
286 | 	}
287 | 	if ca.Compression != nil {
288 | 		rc, err := ca.Compression.OpenReader(sourceArchive)
289 | 		if err != nil {
290 | 			return err
291 | 		}
292 | 		defer rc.Close()
293 | 		sourceArchive = rc
294 | 	}
295 | 	return ca.Extraction.Extract(ctx, sourceArchive, handleFile)
296 | }
297 | 
298 | // MatchResult returns true if the format was matched either
299 | // by name, stream, or both. Name usually refers to matching
300 | // by file extension, and stream usually refers to reading
301 | // the first few bytes of the stream (its header). A stream
302 | // match is generally stronger, as filenames are not always
303 | // indicative of their contents if they even exist at all.
304 | type MatchResult struct {
305 | 	ByName, ByStream bool
306 | }
307 | 
308 | // Matched returns true if a match was made by either name or stream.
309 | func (mr MatchResult) Matched() bool { return mr.ByName || mr.ByStream }
310 | 
311 | func (mr MatchResult) String() string {
312 | 	return fmt.Sprintf("{ByName=%v ByStream=%v}", mr.ByName, mr.ByStream)
313 | }
314 | 
315 | // rewindReader is a Reader that can be rewound (reset) to re-read what
316 | // was already read and then continue to read more from the underlying
317 | // stream. When no more rewinding is necessary, call reader() to get a
318 | // new reader that first reads the buffered bytes, then continues to
319 | // read from the stream. This is useful for "peeking" a stream an
320 | // arbitrary number of bytes. Loosely based on the Connection type
321 | // from https://github.com/mholt/caddy-l4.
322 | //
323 | // If the reader is also an io.Seeker, no buffer is used, and instead
324 | // the stream seeks back to the starting position.
325 | type rewindReader struct {
326 | 	io.Reader
327 | 	start     int64
328 | 	buf       *bytes.Buffer
329 | 	bufReader io.Reader
330 | }
331 | 
332 | func newRewindReader(r io.Reader) (*rewindReader, error) {
333 | 	if r == nil {
334 | 		return nil, nil
335 | 	}
336 | 
337 | 	rr := &rewindReader{Reader: r}
338 | 
339 | 	// avoid buffering if we have a seeker we can use
340 | 	if seeker, ok := r.(io.Seeker); ok {
341 | 		var err error
342 | 		rr.start, err = seeker.Seek(0, io.SeekCurrent)
343 | 		if err != nil {
344 | 			return nil, fmt.Errorf("seek to determine current position: %w", err)
345 | 		}
346 | 	} else {
347 | 		rr.buf = new(bytes.Buffer)
348 | 	}
349 | 
350 | 	return rr, nil
351 | }
352 | 
353 | func (rr *rewindReader) Read(p []byte) (n int, err error) {
354 | 	if rr == nil {
355 | 		panic("reading from nil rewindReader")
356 | 	}
357 | 
358 | 	// if there is a buffer we should read from, start
359 | 	// with that; we only read from the underlying stream
360 | 	// after the buffer has been "depleted"
361 | 	if rr.bufReader != nil {
362 | 		n, err = rr.bufReader.Read(p)
363 | 		if err == io.EOF {
364 | 			rr.bufReader = nil
365 | 			err = nil
366 | 		}
367 | 		if n == len(p) {
368 | 			return
369 | 		}
370 | 	}
371 | 
372 | 	// buffer has been depleted or we are not using one,
373 | 	// so read from underlying stream
374 | 	nr, err := rr.Reader.Read(p[n:])
375 | 
376 | 	// anything that was read needs to be written to
377 | 	// the buffer (if used), even if there was an error
378 | 	if nr > 0 && rr.buf != nil {
379 | 		if nw, errw := rr.buf.Write(p[n : n+nr]); errw != nil {
380 | 			return nw, errw
381 | 		}
382 | 	}
383 | 
384 | 	// up to now, n was how many bytes were read from
385 | 	// the buffer, and nr was how many bytes were read
386 | 	// from the stream; add them to return total count
387 | 	n += nr
388 | 
389 | 	return
390 | }
391 | 
392 | // rewind resets the stream to the beginning by causing
393 | // Read() to start reading from the beginning of the
394 | // stream, or, if buffering, the buffered bytes.
395 | func (rr *rewindReader) rewind() {
396 | 	if rr == nil {
397 | 		return
398 | 	}
399 | 	if ras, ok := rr.Reader.(io.Seeker); ok {
400 | 		if _, err := ras.Seek(rr.start, io.SeekStart); err == nil {
401 | 			return
402 | 		}
403 | 	}
404 | 	rr.bufReader = bytes.NewReader(rr.buf.Bytes())
405 | }
406 | 
407 | // reader returns a reader that reads first from the buffered
408 | // bytes (if buffering), then from the underlying stream; if a
409 | // Seeker, the stream will be seeked back to the start. After
410 | // calling this, no more rewinding is allowed since reads from
411 | // the stream are not recorded, so rewinding properly is impossible.
412 | // If the underlying reader implements io.Seeker, then the
413 | // underlying reader will be used directly.
414 | func (rr *rewindReader) reader() io.Reader {
415 | 	if rr == nil {
416 | 		return nil
417 | 	}
418 | 	if ras, ok := rr.Reader.(io.Seeker); ok {
419 | 		if _, err := ras.Seek(rr.start, io.SeekStart); err == nil {
420 | 			return rr.Reader
421 | 		}
422 | 	}
423 | 	return io.MultiReader(bytes.NewReader(rr.buf.Bytes()), rr.Reader)
424 | }
425 | 
426 | // NoMatch is a special error returned if there are no matching formats.
427 | var NoMatch = fmt.Errorf("no formats matched")
428 | 
429 | // Registered formats.
430 | var formats = make(map[string]Format)
431 | 
432 | // Interface guards
433 | var (
434 | 	_ Format        = (*CompressedArchive)(nil)
435 | 	_ Archiver      = (*CompressedArchive)(nil)
436 | 	_ ArchiverAsync = (*CompressedArchive)(nil)
437 | 	_ Extractor     = (*CompressedArchive)(nil)
438 | 	_ Compressor    = (*CompressedArchive)(nil)
439 | 	_ Decompressor  = (*CompressedArchive)(nil)
440 | )
```

formats_test.go
```
1 | package archives
2 | 
3 | import (
4 | 	"bytes"
5 | 	"context"
6 | 	"errors"
7 | 	"io"
8 | 	"io/fs"
9 | 	"math/rand"
10 | 	"os"
11 | 	"strings"
12 | 	"testing"
13 | 	"time"
14 | )
15 | 
16 | func TestRewindReader(t *testing.T) {
17 | 	data := "the header\nthe body\n"
18 | 
19 | 	r, err := newRewindReader(strings.NewReader(data))
20 | 	if err != nil {
21 | 		t.Errorf("creating rewindReader: %v", err)
22 | 	}
23 | 
24 | 	buf := make([]byte, 10) // enough for 'the header'
25 | 
26 | 	// test rewinding reads
27 | 	for i := 0; i < 10; i++ {
28 | 		r.rewind()
29 | 		n, err := r.Read(buf)
30 | 		if err != nil {
31 | 			t.Errorf("Read failed: %s", err)
32 | 		}
33 | 		if string(buf[:n]) != "the header" {
34 | 			t.Errorf("iteration %d: expected 'the header' but got '%s' (n=%d)", i, string(buf[:n]), n)
35 | 		}
36 | 	}
37 | 
38 | 	// get the reader from header reader and make sure we can read all of the data out
39 | 	r.rewind()
40 | 	finalReader := r.reader()
41 | 	buf = make([]byte, len(data))
42 | 	n, err := io.ReadFull(finalReader, buf)
43 | 	if err != nil {
44 | 		t.Errorf("ReadFull failed: %s (n=%d)", err, n)
45 | 	}
46 | 	if string(buf) != data {
47 | 		t.Errorf("expected '%s' but got '%s'", string(data), string(buf))
48 | 	}
49 | }
50 | 
51 | func TestCompression(t *testing.T) {
52 | 	seed := time.Now().UnixNano()
53 | 	t.Logf("seed: %d", seed)
54 | 	r := rand.New(rand.NewSource(seed))
55 | 
56 | 	contents := make([]byte, 1024)
57 | 	r.Read(contents)
58 | 
59 | 	compressed := new(bytes.Buffer)
60 | 
61 | 	testOK := func(t *testing.T, comp Compression, testFilename string) {
62 | 		// compress into buffer
63 | 		compressed.Reset()
64 | 		wc, err := comp.OpenWriter(compressed)
65 | 		checkErr(t, err, "opening writer")
66 | 		_, err = wc.Write(contents)
67 | 		checkErr(t, err, "writing contents")
68 | 		checkErr(t, wc.Close(), "closing writer")
69 | 
70 | 		// make sure Identify correctly chooses this compression method
71 | 		format, stream, err := Identify(context.Background(), testFilename, compressed)
72 | 		checkErr(t, err, "identifying")
73 | 		if format.Extension() != comp.Extension() {
74 | 			t.Errorf("expected format %s but got %s", comp.Extension(), format.Extension())
75 | 		}
76 | 
77 | 		// read the contents back out and compare
78 | 		decompReader, err := format.(Decompressor).OpenReader(stream)
79 | 		checkErr(t, err, "opening with decompressor '%s'", format.Extension())
80 | 		data, err := io.ReadAll(decompReader)
81 | 		checkErr(t, err, "reading decompressed data")
82 | 		checkErr(t, decompReader.Close(), "closing decompressor")
83 | 		if !bytes.Equal(data, contents) {
84 | 			t.Errorf("not equal to original")
85 | 		}
86 | 	}
87 | 
88 | 	var cannotIdentifyFromStream = map[string]bool{Brotli{}.Extension(): true}
89 | 
90 | 	for _, f := range formats {
91 | 		// only test compressors
92 | 		comp, ok := f.(Compression)
93 | 		if !ok {
94 | 			continue
95 | 		}
96 | 
97 | 		t.Run(f.Extension()+"_with_extension", func(t *testing.T) {
98 | 			testOK(t, comp, "file"+f.Extension())
99 | 		})
100 | 		if !cannotIdentifyFromStream[f.Extension()] {
101 | 			t.Run(f.Extension()+"_without_extension", func(t *testing.T) {
102 | 				testOK(t, comp, "")
103 | 			})
104 | 		}
105 | 	}
106 | }
107 | 
108 | func checkErr(t *testing.T, err error, msgFmt string, args ...any) {
109 | 	t.Helper()
110 | 	if err == nil {
111 | 		return
112 | 	}
113 | 	args = append(args, err)
114 | 	t.Fatalf(msgFmt+": %s", args...)
115 | }
116 | 
117 | func TestIdentifyDoesNotMatchContentFromTrimmedKnownHeaderHaving0Suffix(t *testing.T) {
118 | 	// Using the outcome of `n, err := io.ReadFull(stream, buf)` without minding n
119 | 	// may lead to a mis-characterization for cases with known header ending with 0x0
120 | 	// because the default byte value in a declared array is 0.
121 | 	// This test guards against those cases.
122 | 	tests := []struct {
123 | 		name   string
124 | 		header []byte
125 | 	}{
126 | 		{
127 | 			name:   "rar_v5.0",
128 | 			header: rarHeaderV5_0,
129 | 		},
130 | 		{
131 | 			name:   "rar_v1.5",
132 | 			header: rarHeaderV1_5,
133 | 		},
134 | 		{
135 | 			name:   "xz",
136 | 			header: xzHeader,
137 | 		},
138 | 	}
139 | 	for _, tt := range tests {
140 | 		t.Run(tt.name, func(t *testing.T) {
141 | 			headerLen := len(tt.header)
142 | 			if headerLen == 0 || tt.header[headerLen-1] != 0 {
143 | 				t.Errorf("header expected to end with 0: header=%v", tt.header)
144 | 				return
145 | 			}
146 | 			headerTrimmed := tt.header[:headerLen-1]
147 | 			stream := bytes.NewReader(headerTrimmed)
148 | 			got, _, err := Identify(context.Background(), "", stream)
149 | 			if got != nil {
150 | 				t.Errorf("no Format expected for trimmed know %s header: found Format= %v", tt.name, got.Extension())
151 | 				return
152 | 			}
153 | 			if !errors.Is(err, NoMatch) {
154 | 				t.Errorf("NoMatch expected for for trimmed know %s header: err :=%#v", tt.name, err)
155 | 				return
156 | 			}
157 | 
158 | 		})
159 | 	}
160 | }
161 | 
162 | func TestIdentifyCanAssessSmallOrNoContent(t *testing.T) {
163 | 	type args struct {
164 | 		stream io.ReadSeeker
165 | 	}
166 | 	tests := []struct {
167 | 		name string
168 | 		args args
169 | 	}{
170 | 		{
171 | 			name: "should return nomatch for an empty stream",
172 | 			args: args{
173 | 				stream: bytes.NewReader([]byte{}),
174 | 			},
175 | 		},
176 | 		{
177 | 			name: "should return nomatch for a stream with content size less than known header",
178 | 			args: args{
179 | 				stream: bytes.NewReader([]byte{'a'}),
180 | 			},
181 | 		},
182 | 		{
183 | 			name: "should return nomatch for a stream with content size greater then known header size and not supported format",
184 | 			args: args{
185 | 				stream: bytes.NewReader([]byte(strings.Repeat("this is a txt content", 2))),
186 | 			},
187 | 		},
188 | 	}
189 | 	for _, tt := range tests {
190 | 		t.Run(tt.name, func(t *testing.T) {
191 | 			got, _, err := Identify(context.Background(), "", tt.args.stream)
192 | 			if got != nil {
193 | 				t.Errorf("no Format expected for non archive and not compressed stream: found Format=%#v", got)
194 | 				return
195 | 			}
196 | 			if !errors.Is(err, NoMatch) {
197 | 				t.Errorf("NoMatch expected for non archive and not compressed stream: %#v", err)
198 | 				return
199 | 			}
200 | 
201 | 		})
202 | 	}
203 | }
204 | 
205 | func compress(
206 | 	t *testing.T, compName string, content []byte,
207 | 	openwriter func(w io.Writer) (io.WriteCloser, error),
208 | ) []byte {
209 | 	buf := bytes.NewBuffer(make([]byte, 0, 128))
210 | 	cwriter, err := openwriter(buf)
211 | 	if err != nil {
212 | 		t.Errorf("fail to open compression writer: compression-name=%s, err=%#v", compName, err)
213 | 		return nil
214 | 	}
215 | 	_, err = cwriter.Write(content)
216 | 	if err != nil {
217 | 		cerr := cwriter.Close()
218 | 		t.Errorf(
219 | 			"fail to write using compression writer: compression-name=%s, err=%#v, close-err=%#v",
220 | 			compName, err, cerr)
221 | 		return nil
222 | 	}
223 | 	err = cwriter.Close()
224 | 	if err != nil {
225 | 		t.Errorf("fail to close compression writer: compression-name=%s, err=%#v", compName, err)
226 | 		return nil
227 | 	}
228 | 	return buf.Bytes()
229 | }
230 | 
231 | func archive(t *testing.T, arch Archiver, fname string, fileInfo fs.FileInfo) []byte {
232 | 	files := []FileInfo{
233 | 		{FileInfo: fileInfo, NameInArchive: "tmp.txt",
234 | 			Open: func() (fs.File, error) {
235 | 				return os.Open(fname)
236 | 			}},
237 | 	}
238 | 	buf := bytes.NewBuffer(make([]byte, 0, 128))
239 | 	err := arch.Archive(context.TODO(), buf, files)
240 | 	if err != nil {
241 | 		t.Errorf("fail to create archive: err=%#v", err)
242 | 		return nil
243 | 	}
244 | 	return buf.Bytes()
245 | 
246 | }
247 | 
248 | type writeNopCloser struct{ io.Writer }
249 | 
250 | func (wnc writeNopCloser) Close() error { return nil }
251 | 
252 | func newWriteNopCloser(w io.Writer) (io.WriteCloser, error) {
253 | 	return writeNopCloser{w}, nil
254 | }
255 | 
256 | func newTmpTextFile(t *testing.T, content string) (string, fs.FileInfo) {
257 | 	tmpTxtFile, err := os.CreateTemp("", "TestIdentifyFindFormatByStreamContent-tmp-*.txt")
258 | 	if err != nil {
259 | 		t.Errorf("fail to create tmp test file for archive tests: err=%v", err)
260 | 		return "", nil
261 | 	}
262 | 	fname := tmpTxtFile.Name()
263 | 
264 | 	if _, err = tmpTxtFile.Write([]byte(content)); err != nil {
265 | 		t.Errorf("fail to write content to tmp-txt-file: err=%#v", err)
266 | 		return "", nil
267 | 	}
268 | 	if err = tmpTxtFile.Close(); err != nil {
269 | 		t.Errorf("fail to close tmp-txt-file: err=%#v", err)
270 | 		return "", nil
271 | 	}
272 | 	fi, err := os.Stat(fname)
273 | 	if err != nil {
274 | 		t.Errorf("fail to get tmp-txt-file stats: err=%v", err)
275 | 		return "", nil
276 | 	}
277 | 
278 | 	return fname, fi
279 | }
280 | 
281 | func TestIdentifyFindFormatByStreamContent(t *testing.T) {
282 | 	tmpTxtFileName, tmpTxtFileInfo := newTmpTextFile(t, "this is text that has to be long enough for brotli to match")
283 | 	t.Cleanup(func() {
284 | 		os.RemoveAll(tmpTxtFileName)
285 | 	})
286 | 
287 | 	tests := []struct {
288 | 		name                  string
289 | 		content               []byte
290 | 		openCompressionWriter func(w io.Writer) (io.WriteCloser, error)
291 | 		compressorName        string
292 | 		wantFormatName        string
293 | 	}{
294 | 		{
295 | 			name:                  "should recognize brotli",
296 | 			openCompressionWriter: Brotli{}.OpenWriter,
297 | 			content:               []byte("this is text, but it has to be long enough to match brotli which doesn't have a magic number"),
298 | 			compressorName:        ".br",
299 | 			wantFormatName:        ".br",
300 | 		},
301 | 		{
302 | 			name:                  "should recognize bz2",
303 | 			openCompressionWriter: Bz2{}.OpenWriter,
304 | 			content:               []byte("this is text"),
305 | 			compressorName:        ".bz2",
306 | 			wantFormatName:        ".bz2",
307 | 		},
308 | 		{
309 | 			name:                  "should recognize gz",
310 | 			openCompressionWriter: Gz{}.OpenWriter,
311 | 			content:               []byte("this is text"),
312 | 			compressorName:        ".gz",
313 | 			wantFormatName:        ".gz",
314 | 		},
315 | 		{
316 | 			name:                  "should recognize lz4",
317 | 			openCompressionWriter: Lz4{}.OpenWriter,
318 | 			content:               []byte("this is text"),
319 | 			compressorName:        ".lz4",
320 | 			wantFormatName:        ".lz4",
321 | 		},
322 | 		{
323 | 			name:                  "should recognize lz",
324 | 			openCompressionWriter: Lzip{}.OpenWriter,
325 | 			content:               []byte("this is text"),
326 | 			compressorName:        ".lz",
327 | 			wantFormatName:        ".lz",
328 | 		},
329 | 		{
330 | 			name:                  "should recognize sz",
331 | 			openCompressionWriter: Sz{}.OpenWriter,
332 | 			content:               []byte("this is text"),
333 | 			compressorName:        ".sz",
334 | 			wantFormatName:        ".sz",
335 | 		},
336 | 		{
337 | 			name:                  "should recognize xz",
338 | 			openCompressionWriter: Xz{}.OpenWriter,
339 | 			content:               []byte("this is text"),
340 | 			compressorName:        ".xz",
341 | 			wantFormatName:        ".xz",
342 | 		},
343 | 		{
344 | 			name:                  "should recognize zst",
345 | 			openCompressionWriter: Zstd{}.OpenWriter,
346 | 			content:               []byte("this is text"),
347 | 			compressorName:        ".zst",
348 | 			wantFormatName:        ".zst",
349 | 		},
350 | 		{
351 | 			name:                  "should recognize tar",
352 | 			openCompressionWriter: newWriteNopCloser,
353 | 			content:               archive(t, Tar{}, tmpTxtFileName, tmpTxtFileInfo),
354 | 			compressorName:        "",
355 | 			wantFormatName:        ".tar",
356 | 		},
357 | 		{
358 | 			name:                  "should recognize tar.gz",
359 | 			openCompressionWriter: Gz{}.OpenWriter,
360 | 			content:               archive(t, Tar{}, tmpTxtFileName, tmpTxtFileInfo),
361 | 			compressorName:        ".gz",
362 | 			wantFormatName:        ".tar.gz",
363 | 		},
364 | 		{
365 | 			name:                  "should recognize zip",
366 | 			openCompressionWriter: newWriteNopCloser,
367 | 			content:               archive(t, Zip{}, tmpTxtFileName, tmpTxtFileInfo),
368 | 			compressorName:        "",
369 | 			wantFormatName:        ".zip",
370 | 		},
371 | 		{
372 | 			name:                  "should recognize rar by v5.0 header",
373 | 			openCompressionWriter: newWriteNopCloser,
374 | 			content:               rarHeaderV5_0[:],
375 | 			compressorName:        "",
376 | 			wantFormatName:        ".rar",
377 | 		},
378 | 		{
379 | 			name:                  "should recognize rar by v1.5 header",
380 | 			openCompressionWriter: newWriteNopCloser,
381 | 			content:               rarHeaderV1_5[:],
382 | 			compressorName:        "",
383 | 			wantFormatName:        ".rar",
384 | 		},
385 | 		{
386 | 			name:                  "should recognize zz",
387 | 			openCompressionWriter: Zlib{}.OpenWriter,
388 | 			content:               []byte("this is text"),
389 | 			compressorName:        ".zz",
390 | 			wantFormatName:        ".zz",
391 | 		},
392 | 	}
393 | 	for _, tt := range tests {
394 | 		t.Run(tt.name, func(t *testing.T) {
395 | 			stream := bytes.NewReader(compress(t, tt.compressorName, tt.content, tt.openCompressionWriter))
396 | 			got, _, err := Identify(context.Background(), "", stream)
397 | 			if err != nil {
398 | 				t.Errorf("should have found a corresponding Format, but got err=%+v", err)
399 | 				return
400 | 			}
401 | 			if tt.wantFormatName != got.Extension() {
402 | 				t.Errorf("unexpected format found: expected=%s actual=%s", tt.wantFormatName, got.Extension())
403 | 				return
404 | 			}
405 | 
406 | 		})
407 | 	}
408 | }
409 | 
410 | func TestIdentifyAndOpenZip(t *testing.T) {
411 | 	f, err := os.Open("testdata/test.zip")
412 | 	checkErr(t, err, "opening zip")
413 | 	defer f.Close()
414 | 
415 | 	format, reader, err := Identify(context.Background(), "test.zip", f)
416 | 	checkErr(t, err, "identifying zip")
417 | 	if format.Extension() != ".zip" {
418 | 		t.Errorf("unexpected format found: expected=.zip actual=%s", format.Extension())
419 | 	}
420 | 
421 | 	err = format.(Extractor).Extract(context.Background(), reader, func(ctx context.Context, f FileInfo) error {
422 | 		rc, err := f.Open()
423 | 		if err != nil {
424 | 			return err
425 | 		}
426 | 		defer rc.Close()
427 | 		_, err = io.ReadAll(rc)
428 | 		return err
429 | 	})
430 | 	checkErr(t, err, "extracting zip")
431 | }
432 | 
433 | func TestIdentifyASCIIFileStartingWithX(t *testing.T) {
434 | 	// Create a temporary file starting with the letter 'x'
435 | 	tmpFile, err := os.CreateTemp("", "TestIdentifyASCIIFileStartingWithX-tmp-*.txt")
436 | 	if err != nil {
437 | 		t.Errorf("fail to create tmp test file for archive tests: err=%v", err)
438 | 	}
439 | 	defer os.Remove(tmpFile.Name())
440 | 
441 | 	_, err = tmpFile.Write([]byte("xThis is a test file"))
442 | 	if err != nil {
443 | 		t.Errorf("Failed to write to temp file: %v", err)
444 | 	}
445 | 	tmpFile.Close()
446 | 
447 | 	// Open the file and use the Identify function
448 | 	file, err := os.Open(tmpFile.Name())
449 | 	if err != nil {
450 | 		t.Errorf("Failed to open temp file: %v", err)
451 | 	}
452 | 	defer file.Close()
453 | 
454 | 	_, _, err = Identify(context.Background(), tmpFile.Name(), file)
455 | 	if !errors.Is(err, NoMatch) {
456 | 		t.Errorf("Identify failed: %v", err)
457 | 	}
458 | }
459 | 
460 | func TestIdentifyStreamNil(t *testing.T) {
461 | 	format, _, err := Identify(context.Background(), "test.tar.zst", nil)
462 | 	checkErr(t, err, "identifying tar.zst")
463 | 	if format.Extension() != ".tar.zst" {
464 | 		t.Errorf("unexpected format found: expected=.tar.zst actual=%s", format.Extension())
465 | 	}
466 | }
```

fs.go
```
1 | package archives
2 | 
3 | import (
4 | 	"context"
5 | 	"errors"
6 | 	"fmt"
7 | 	"io"
8 | 	"io/fs"
9 | 	"os"
10 | 	"path"
11 | 	"path/filepath"
12 | 	"runtime"
13 | 	"slices"
14 | 	"strings"
15 | 	"sync"
16 | 	"time"
17 | )
18 | 
19 | // FileSystem identifies the format of the input and returns a read-only file system.
20 | // The input can be a filename, stream, or both.
21 | //
22 | // If only a filename is specified, it may be a path to a directory, archive file,
23 | // compressed archive file, compressed regular file, or any other regular file on
24 | // disk. If the filename is a directory, its contents are accessed directly from
25 | // the device's file system. If the filename is an archive file, the contents can
26 | // be accessed like a normal directory; compressed archive files are transparently
27 | // decompressed as contents are accessed. And if the filename is any other file, it
28 | // is the only file in the returned file system; if the file is compressed, it is
29 | // transparently decompressed when read from.
30 | //
31 | // If a stream is specified, the filename (if available) is used as a hint to help
32 | // identify its format. Streams of archive files must be able to be made into an
33 | // io.SectionReader (for safe concurrency) which requires io.ReaderAt and io.Seeker
34 | // (to efficiently determine size). The automatic format identification requires
35 | // io.Reader and will use io.Seeker if supported to avoid buffering.
36 | //
37 | // Whether the data comes from disk or a stream, it is peeked at to automatically
38 | // detect which format to use.
39 | //
40 | // This function essentially offers uniform read access to various kinds of files:
41 | // directories, archives, compressed archives, individual files, and file streams
42 | // are all treated the same way.
43 | //
44 | // NOTE: The performance of compressed tar archives is not great due to overhead
45 | // with decompression. However, the fs.WalkDir() use case has been optimized to
46 | // create an index on first call to ReadDir().
47 | func FileSystem(ctx context.Context, filename string, stream ReaderAtSeeker) (fs.FS, error) {
48 | 	if filename == "" && stream == nil {
49 | 		return nil, errors.New("no input")
50 | 	}
51 | 
52 | 	// if an input stream is specified, we'll use that for identification
53 | 	// and for ArchiveFS (if it's an archive); but if not, we'll open the
54 | 	// file and read it for identification, but in that case we won't want
55 | 	// to also use it for the ArchiveFS (because we need to close what we
56 | 	// opened, and ArchiveFS opens its own files), hence this separate var
57 | 	idStream := stream
58 | 
59 | 	// if input is only a filename (no stream), check if it's a directory;
60 | 	// if not, open it so we can determine which format to use (filename
61 | 	// is not always a good indicator of file format)
62 | 	if filename != "" && stream == nil {
63 | 		info, err := os.Stat(filename)
64 | 		if err != nil {
65 | 			return nil, err
66 | 		}
67 | 
68 | 		// real folders can be accessed easily
69 | 		if info.IsDir() {
70 | 			return DirFS(filename), nil
71 | 		}
72 | 
73 | 		// if any archive formats recognize this file, access it like a folder
74 | 		file, err := os.Open(filename)
75 | 		if err != nil {
76 | 			return nil, err
77 | 		}
78 | 		defer file.Close()
79 | 		idStream = file // use file for format identification only
80 | 	}
81 | 
82 | 	// normally, callers should use the Reader value returned from Identify, but
83 | 	// our input is a Seeker, so we know the original input value gets returned
84 | 	format, _, err := Identify(ctx, filepath.Base(filename), idStream)
85 | 	if errors.Is(err, NoMatch) {
86 | 		return FileFS{Path: filename}, nil // must be an ordinary file
87 | 	}
88 | 	if err != nil {
89 | 		return nil, fmt.Errorf("identify format: %w", err)
90 | 	}
91 | 
92 | 	switch fileFormat := format.(type) {
93 | 	case Extractor:
94 | 		// if no stream was input, return an ArchiveFS that relies on the filepath
95 | 		if stream == nil {
96 | 			return &ArchiveFS{Path: filename, Format: fileFormat, Context: ctx}, nil
97 | 		}
98 | 
99 | 		// otherwise, if a stream was input, return an ArchiveFS that relies on that
100 | 
101 | 		// determine size -- we know that the stream value we get back from
102 | 		// Identify is the same type as what we input because it is a Seeker
103 | 		size, err := streamSizeBySeeking(stream)
104 | 		if err != nil {
105 | 			return nil, fmt.Errorf("seeking for size: %w", err)
106 | 		}
107 | 
108 | 		sr := io.NewSectionReader(stream, 0, size)
109 | 
110 | 		return &ArchiveFS{Stream: sr, Format: fileFormat, Context: ctx}, nil
111 | 
112 | 	case Compression:
113 | 		return FileFS{Path: filename, Compression: fileFormat}, nil
114 | 	}
115 | 
116 | 	return nil, fmt.Errorf("unable to create file system rooted at %s due to unsupported file or folder type", filename)
117 | }
118 | 
119 | // ReaderAtSeeker is a type that can read, read at, and seek.
120 | // os.File and io.SectionReader both implement this interface.
121 | type ReaderAtSeeker interface {
122 | 	io.Reader
123 | 	io.ReaderAt
124 | 	io.Seeker
125 | }
126 | 
127 | // FileFS allows accessing a file on disk using a consistent file system interface.
128 | // The value should be the path to a regular file, not a directory. This file will
129 | // be the only entry in the file system and will be at its root. It can be accessed
130 | // within the file system by the name of "." or the filename.
131 | //
132 | // If the file is compressed, set the Compression field so that reads from the
133 | // file will be transparently decompressed.
134 | type FileFS struct {
135 | 	// The path to the file on disk.
136 | 	Path string
137 | 
138 | 	// If file is compressed, setting this field will
139 | 	// transparently decompress reads.
140 | 	Compression Decompressor
141 | }
142 | 
143 | // Open opens the named file, which must be the file used to create the file system.
144 | func (f FileFS) Open(name string) (fs.File, error) {
145 | 	if err := f.checkName(name, "open"); err != nil {
146 | 		return nil, err
147 | 	}
148 | 	file, err := os.Open(f.Path)
149 | 	if err != nil {
150 | 		return nil, err
151 | 	}
152 | 	if f.Compression == nil {
153 | 		return file, nil
154 | 	}
155 | 	r, err := f.Compression.OpenReader(file)
156 | 	if err != nil {
157 | 		return nil, err
158 | 	}
159 | 	return compressedFile{r, closeBoth{file, r}}, nil
160 | }
161 | 
162 | // Stat stats the named file, which must be the file used to create the file system.
163 | func (f FileFS) Stat(name string) (fs.FileInfo, error) {
164 | 	if err := f.checkName(name, "stat"); err != nil {
165 | 		return nil, err
166 | 	}
167 | 	return os.Stat(f.Path)
168 | }
169 | 
170 | // ReadDir returns a directory listing with the file as the singular entry.
171 | func (f FileFS) ReadDir(name string) ([]fs.DirEntry, error) {
172 | 	if err := f.checkName(name, "stat"); err != nil {
173 | 		return nil, err
174 | 	}
175 | 	info, err := f.Stat(name)
176 | 	if err != nil {
177 | 		return nil, err
178 | 	}
179 | 	return []fs.DirEntry{fs.FileInfoToDirEntry(info)}, nil
180 | }
181 | 
182 | // checkName ensures the name is a valid path and also, in the case of
183 | // the FileFS, that it is either ".", the filename originally passed in
184 | // to create the FileFS, or the base of the filename (name without path).
185 | // Other names do not make sense for a FileFS since the FS is only 1 file.
186 | func (f FileFS) checkName(name, op string) error {
187 | 	if name == f.Path {
188 | 		return nil
189 | 	}
190 | 	if !fs.ValidPath(name) {
191 | 		return &fs.PathError{Op: op, Path: name, Err: fs.ErrInvalid}
192 | 	}
193 | 	if name != "." && name != filepath.Base(f.Path) {
194 | 		return &fs.PathError{Op: op, Path: name, Err: fs.ErrNotExist}
195 | 	}
196 | 	return nil
197 | }
198 | 
199 | // compressedFile is an fs.File that specially reads
200 | // from a decompression reader, and which closes both
201 | // that reader and the underlying file.
202 | type compressedFile struct {
203 | 	io.Reader // decompressor
204 | 	closeBoth // file and decompressor
205 | }
206 | 
207 | // DirFS is similar to os.dirFS (obtained via os.DirFS()), but it is
208 | // exported so it can be used with type assertions. It also returns
209 | // FileInfo/DirEntry values where Name() always returns the name of
210 | // the directory instead of ".". This type does not guarantee any
211 | // sort of sandboxing.
212 | type DirFS string
213 | 
214 | // Open opens the named file.
215 | func (d DirFS) Open(name string) (fs.File, error) {
216 | 	if err := d.checkName(name, "open"); err != nil {
217 | 		return nil, err
218 | 	}
219 | 	return os.Open(filepath.Join(string(d), name))
220 | }
221 | 
222 | // ReadDir returns a listing of all the files in the named directory.
223 | func (d DirFS) ReadDir(name string) ([]fs.DirEntry, error) {
224 | 	if err := d.checkName(name, "readdir"); err != nil {
225 | 		return nil, err
226 | 	}
227 | 	return os.ReadDir(filepath.Join(string(d), name))
228 | }
229 | 
230 | // Stat returns info about the named file.
231 | func (d DirFS) Stat(name string) (fs.FileInfo, error) {
232 | 	if err := d.checkName(name, "stat"); err != nil {
233 | 		return nil, err
234 | 	}
235 | 	info, err := os.Stat(filepath.Join(string(d), name))
236 | 	if err != nil {
237 | 		return info, err
238 | 	}
239 | 	if info.Name() == "." {
240 | 		info = dotFileInfo{info, filepath.Base(string(d))}
241 | 	}
242 | 	return info, nil
243 | }
244 | 
245 | // Sub returns an FS corresponding to the subtree rooted at dir.
246 | func (d DirFS) Sub(dir string) (fs.FS, error) {
247 | 	if err := d.checkName(dir, "sub"); err != nil {
248 | 		return nil, err
249 | 	}
250 | 	info, err := d.Stat(dir)
251 | 	if err != nil {
252 | 		return nil, err
253 | 	}
254 | 	if !info.IsDir() {
255 | 		return nil, fmt.Errorf("%s is not a directory", dir)
256 | 	}
257 | 	return DirFS(filepath.Join(string(d), dir)), nil
258 | }
259 | 
260 | // checkName returns an error if name is not a valid path according to the docs of
261 | // the io/fs package, with an extra cue taken from the standard lib's implementation
262 | // of os.dirFS.Open(), which checks for invalid characters in Windows paths.
263 | func (DirFS) checkName(name, op string) error {
264 | 	if !fs.ValidPath(name) || runtime.GOOS == "windows" && strings.ContainsAny(name, `\:`) {
265 | 		return &fs.PathError{Op: op, Path: name, Err: fs.ErrInvalid}
266 | 	}
267 | 	return nil
268 | }
269 | 
270 | // ArchiveFS allows reading an archive (or a compressed archive) using a
271 | // consistent file system interface. Essentially, it allows traversal and
272 | // reading of archive contents the same way as any normal directory on disk.
273 | // The contents of compressed archives are transparently decompressed.
274 | //
275 | // A valid ArchiveFS value must set either Path or Stream, but not both.
276 | // If Path is set, a literal file will be opened from the disk.
277 | // If Stream is set, new SectionReaders will be implicitly created to
278 | // access the stream, enabling safe, concurrent access.
279 | //
280 | // NOTE: Due to Go's file system APIs (see package io/fs), the performance
281 | // of ArchiveFS can suffer when using fs.WalkDir(). To mitigate this,
282 | // an optimized fs.ReadDirFS has been implemented that indexes the entire
283 | // archive on the first call to ReadDir() (since the entire archive needs
284 | // to be walked for every call to ReadDir() anyway, as archive contents are
285 | // often unordered). The first call to ReadDir(), i.e. near the start of the
286 | // walk, will be slow for large archives, but should be instantaneous after.
287 | // If you don't care about walking a file system in directory order, consider
288 | // calling Extract() on the underlying archive format type directly, which
289 | // walks the archive in entry order, without needing to do any sorting.
290 | //
291 | // Note that fs.FS implementations, including this one, reject paths starting
292 | // with "./". This can be problematic sometimes, as it is not uncommon for
293 | // tarballs to contain a top-level/root directory literally named ".", which
294 | // can happen if a tarball is created in the same directory it is archiving.
295 | // The underlying Extract() calls are faithful to entries with this name,
296 | // but file systems have certain semantics around "." that restrict its use.
297 | // For example, a file named "." cannot be created on a real file system
298 | // because it is a special name that means "current directory".
299 | //
300 | // We had to decide whether to honor the true name in the archive, or honor
301 | // file system semantics. Given that this is a virtual file system and other
302 | // code using the fs.FS APIs will trip over a literal directory named ".",
303 | // we choose to honor file system semantics. Files named "." are ignored;
304 | // directories with this name are effectively transparent; their contents
305 | // get promoted up a directory/level. This means a file at "./x" where "."
306 | // is a literal directory name, its name will be passed in as "x" in
307 | // WalkDir callbacks. If you need the raw, uninterpeted values from an
308 | // archive, use the formats' Extract() method directly. See
309 | // https://github.com/golang/go/issues/70155 for a little more background.
310 | //
311 | // This does have one negative edge case... a tar containing contents like
312 | // [x . ./x] will have a conflict on the file named "x" because "./x" will
313 | // also be accessed with the name of "x".
314 | type ArchiveFS struct {
315 | 	// set one of these
316 | 	Path   string            // path to the archive file on disk, or...
317 | 	Stream *io.SectionReader // ...stream from which to read archive
318 | 
319 | 	Format  Extractor       // the archive format
320 | 	Prefix  string          // optional subdirectory in which to root the fs
321 | 	Context context.Context // optional; mainly for cancellation
322 | 
323 | 	// amortizing cache speeds up walks (esp. ReadDir)
324 | 	contents map[string]fs.FileInfo
325 | 	dirs     map[string][]fs.DirEntry
326 | }
327 | 
328 | // context always return a context, preferring f.Context if not nil.
329 | func (f ArchiveFS) context() context.Context {
330 | 	if f.Context != nil {
331 | 		return f.Context
332 | 	}
333 | 	return context.Background()
334 | }
335 | 
336 | // Open opens the named file from within the archive. If name is "." then
337 | // the archive file itself will be opened as a directory file.
338 | func (f ArchiveFS) Open(name string) (fs.File, error) {
339 | 	if !fs.ValidPath(name) {
340 | 		return nil, &fs.PathError{Op: "open", Path: name, Err: fmt.Errorf("%w: %s", fs.ErrInvalid, name)}
341 | 	}
342 | 
343 | 	// apply prefix if fs is rooted in a subtree
344 | 	name = path.Join(f.Prefix, name)
345 | 
346 | 	// if we've already indexed the archive, we can know quickly if the file doesn't exist,
347 | 	// and we can also return directory files with their entries instantly
348 | 	if f.contents != nil {
349 | 		if info, found := f.contents[name]; found {
350 | 			if info.IsDir() {
351 | 				if entries, ok := f.dirs[name]; ok {
352 | 					return &dirFile{info: info, entries: entries}, nil
353 | 				}
354 | 			}
355 | 		} else {
356 | 			if entries, found := f.dirs[name]; found {
357 | 				return &dirFile{info: implicitDirInfo{implicitDirEntry{name}}, entries: entries}, nil
358 | 			}
359 | 			return nil, &fs.PathError{Op: "open", Path: name, Err: fmt.Errorf("open %s: %w", name, fs.ErrNotExist)}
360 | 		}
361 | 	}
362 | 
363 | 	// if a filename is specified, open the archive file
364 | 	var archiveFile *os.File
365 | 	var err error
366 | 	if f.Stream == nil {
367 | 		archiveFile, err = os.Open(f.Path)
368 | 		if err != nil {
369 | 			return nil, err
370 | 		}
371 | 		defer func() {
372 | 			// close the archive file if extraction failed; we can only
373 | 			// count on the user/caller closing it if they successfully
374 | 			// got the handle to the extracted file
375 | 			if err != nil {
376 | 				archiveFile.Close()
377 | 			}
378 | 		}()
379 | 	} else if f.Stream == nil {
380 | 		return nil, fmt.Errorf("no input; one of Path or Stream must be set")
381 | 	}
382 | 
383 | 	// handle special case of opening the archive root
384 | 	if name == "." {
385 | 		var archiveInfo fs.FileInfo
386 | 		if archiveFile != nil {
387 | 			archiveInfo, err = archiveFile.Stat()
388 | 			if err != nil {
389 | 				return nil, err
390 | 			}
391 | 		} else {
392 | 			archiveInfo = implicitDirInfo{
393 | 				implicitDirEntry{"."},
394 | 			}
395 | 		}
396 | 		var entries []fs.DirEntry
397 | 		entries, err = f.ReadDir(name)
398 | 		if err != nil {
399 | 			return nil, err
400 | 		}
401 | 		if archiveFile != nil {
402 | 			// the archiveFile is closed at return only if there's an
403 | 			// error; in this case, though, we can close it regardless
404 | 			if err := archiveFile.Close(); err != nil {
405 | 				return nil, err
406 | 			}
407 | 		}
408 | 		return &dirFile{
409 | 			info:    dirFileInfo{archiveInfo},
410 | 			entries: entries,
411 | 		}, nil
412 | 	}
413 | 
414 | 	var inputStream io.Reader
415 | 	if f.Stream == nil {
416 | 		inputStream = archiveFile
417 | 	} else {
418 | 		inputStream = io.NewSectionReader(f.Stream, 0, f.Stream.Size())
419 | 	}
420 | 
421 | 	var decompressor io.ReadCloser
422 | 	if decomp, ok := f.Format.(Decompressor); ok && decomp != nil {
423 | 		decompressor, err = decomp.OpenReader(inputStream)
424 | 		if err != nil {
425 | 			return nil, err
426 | 		}
427 | 		inputStream = decompressor
428 | 	}
429 | 
430 | 	// prepare the handler that we'll need if we have to iterate the
431 | 	// archive to find the file being requested
432 | 	var fsFile fs.File
433 | 	handler := func(ctx context.Context, file FileInfo) error {
434 | 		if err := ctx.Err(); err != nil {
435 | 			return err
436 | 		}
437 | 
438 | 		// paths in archives can't necessarily be trusted; also clean up any "./" prefix
439 | 		file.NameInArchive = path.Clean(file.NameInArchive)
440 | 
441 | 		// ignore this entry if it's neither the file we're looking for, nor
442 | 		// one of its descendents; we can't just check that the filename is
443 | 		// a prefix of the requested file, because that could wrongly match
444 | 		// "a/b/c.jpg.json" if the requested filename is "a/b/c.jpg", and
445 | 		// this could result in loading the wrong file (!!) so we append a
446 | 		// path separator to ensure that can't happen: "a/b/c.jpg.json/"
447 | 		// is not prefixed by "a/b/c.jpg/", but it will still match as we
448 | 		// expect: "a/b/c/d/" is is prefixed by "a/b/c/", allowing us to
449 | 		// match descenedent files, and "a/b/c.jpg/" is prefixed by
450 | 		// "a/b/c.jpg/", allowing us to match exact filenames.
451 | 		if !strings.HasPrefix(file.NameInArchive+"/", name+"/") {
452 | 			return nil
453 | 		}
454 | 
455 | 		// if this is the requested file, and it's a directory, set up the dirFile,
456 | 		// which will include a listing of all its contents as we continue iterating
457 | 		if file.NameInArchive == name && file.IsDir() {
458 | 			fsFile = &dirFile{info: file} // will fill entries slice as we continue iterating
459 | 			return nil
460 | 		}
461 | 
462 | 		// if the named file was a directory and we are filling its entries,
463 | 		// add this entry to the list
464 | 		if df, ok := fsFile.(*dirFile); ok {
465 | 			df.entries = append(df.entries, fs.FileInfoToDirEntry(file))
466 | 
467 | 			// don't traverse into subfolders
468 | 			if file.IsDir() {
469 | 				return fs.SkipDir
470 | 			}
471 | 
472 | 			return nil
473 | 		}
474 | 
475 | 		innerFile, err := file.Open()
476 | 		if err != nil {
477 | 			return err
478 | 		}
479 | 
480 | 		fsFile = innerFile
481 | 		if archiveFile != nil {
482 | 			fsFile = closeBoth{File: innerFile, c: archiveFile}
483 | 		}
484 | 
485 | 		if decompressor != nil {
486 | 			fsFile = closeBoth{fsFile, decompressor}
487 | 		}
488 | 
489 | 		return fs.SkipAll
490 | 	}
491 | 
492 | 	// when we start the walk, we pass in a nil list of files to extract, since
493 | 	// files may have a "." component in them, and the underlying format doesn't
494 | 	// know about our file system semantics, so we need to filter ourselves (it's
495 | 	// not significantly less efficient).
496 | 	if ar, ok := f.Format.(CompressedArchive); ok {
497 | 		// bypass the CompressedArchive format's opening of the decompressor, since
498 | 		// we already did it because we need to keep it open after returning.
499 | 		// "I BYPASSED THE COMPRESSOR!" -Rey
500 | 		err = ar.Extraction.Extract(f.context(), inputStream, handler)
501 | 	} else {
502 | 		err = f.Format.Extract(f.context(), inputStream, handler)
503 | 	}
504 | 	if err != nil {
505 | 		return nil, &fs.PathError{Op: "open", Path: name, Err: fmt.Errorf("extract: %w", err)}
506 | 	}
507 | 	if fsFile == nil {
508 | 		return nil, &fs.PathError{Op: "open", Path: name, Err: fmt.Errorf("open %s: %w", name, fs.ErrNotExist)}
509 | 	}
510 | 
511 | 	return fsFile, nil
512 | }
513 | 
514 | // Stat stats the named file from within the archive. If name is "." then
515 | // the archive file itself is statted and treated as a directory file.
516 | func (f ArchiveFS) Stat(name string) (fs.FileInfo, error) {
517 | 	if !fs.ValidPath(name) {
518 | 		return nil, &fs.PathError{Op: "stat", Path: name, Err: fmt.Errorf("%s: %w", name, fs.ErrInvalid)}
519 | 	}
520 | 
521 | 	if name == "." {
522 | 		if f.Path != "" {
523 | 			fileInfo, err := os.Stat(f.Path)
524 | 			if err != nil {
525 | 				return nil, &fs.PathError{Op: "stat", Path: name, Err: fmt.Errorf("stat(a) %s: %w", name, err)}
526 | 			}
527 | 			return dirFileInfo{fileInfo}, nil
528 | 		} else if f.Stream != nil {
529 | 			return implicitDirInfo{implicitDirEntry{name}}, nil
530 | 		}
531 | 	}
532 | 
533 | 	// apply prefix if fs is rooted in a subtree
534 | 	name = path.Join(f.Prefix, name)
535 | 
536 | 	// if archive has already been indexed, simply use it
537 | 	if f.contents != nil {
538 | 		if info, ok := f.contents[name]; ok {
539 | 			return info, nil
540 | 		}
541 | 		return nil, &fs.PathError{Op: "stat", Path: name, Err: fmt.Errorf("stat(b) %s: %w", name, fs.ErrNotExist)}
542 | 	}
543 | 
544 | 	var archiveFile *os.File
545 | 	var err error
546 | 	if f.Stream == nil {
547 | 		archiveFile, err = os.Open(f.Path)
548 | 		if err != nil {
549 | 			return nil, &fs.PathError{Op: "stat", Path: name, Err: fmt.Errorf("stat(c) %s: %w", name, err)}
550 | 		}
551 | 		defer archiveFile.Close()
552 | 	}
553 | 
554 | 	var result FileInfo
555 | 	var fallback fs.FileInfo // possibly needed if only an implied directory
556 | 	handler := func(ctx context.Context, file FileInfo) error {
557 | 		if err := ctx.Err(); err != nil {
558 | 			return err
559 | 		}
560 | 		cleanName := path.Clean(file.NameInArchive)
561 | 		if cleanName == name {
562 | 			result = file
563 | 			return fs.SkipAll
564 | 		}
565 | 		// it's possible the requested name is an implicit directory;
566 | 		// remember if we see it along the way, just in case
567 | 		if fallback == nil && strings.HasPrefix(cleanName, name) {
568 | 			fallback = implicitDirInfo{implicitDirEntry{name}}
569 | 		}
570 | 		return nil
571 | 	}
572 | 	var inputStream io.Reader = archiveFile
573 | 	if f.Stream != nil {
574 | 		inputStream = io.NewSectionReader(f.Stream, 0, f.Stream.Size())
575 | 	}
576 | 	err = f.Format.Extract(f.context(), inputStream, handler)
577 | 	if err != nil && result.FileInfo == nil {
578 | 		return nil, &fs.PathError{Op: "stat", Path: name, Err: fmt.Errorf("stat(d) %s: %w", name, fs.ErrNotExist)}
579 | 	}
580 | 	if result.FileInfo == nil {
581 | 		// looks like the requested name does not exist in the archive,
582 | 		// but we can return some basic info if it was an implicit directory
583 | 		if fallback != nil {
584 | 			return fallback, nil
585 | 		}
586 | 		return nil, &fs.PathError{Op: "stat", Path: name, Err: fmt.Errorf("stat(e) %s: %w", name, fs.ErrNotExist)}
587 | 	}
588 | 	return result.FileInfo, nil
589 | }
590 | 
591 | // ReadDir reads the named directory from within the archive. If name is "."
592 | // then the root of the archive content is listed.
593 | func (f *ArchiveFS) ReadDir(name string) ([]fs.DirEntry, error) {
594 | 	if !fs.ValidPath(name) {
595 | 		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
596 | 	}
597 | 
598 | 	// apply prefix if fs is rooted in a subtree
599 | 	name = path.Join(f.Prefix, name)
600 | 
601 | 	// fs.WalkDir() calls ReadDir() once per directory, and for archives with
602 | 	// lots of directories, that is very slow, since we have to traverse the
603 | 	// entire archive in order to ensure that we got all the entries for a
604 | 	// directory -- so we can fast-track this lookup if we've done the
605 | 	// traversal already
606 | 	if len(f.dirs) > 0 {
607 | 		return f.dirs[name], nil
608 | 	}
609 | 
610 | 	f.contents = make(map[string]fs.FileInfo)
611 | 	f.dirs = make(map[string][]fs.DirEntry)
612 | 
613 | 	var archiveFile *os.File
614 | 	var err error
615 | 	if f.Stream == nil {
616 | 		archiveFile, err = os.Open(f.Path)
617 | 		if err != nil {
618 | 			return nil, err
619 | 		}
620 | 		defer archiveFile.Close()
621 | 	}
622 | 
623 | 	handler := func(ctx context.Context, file FileInfo) error {
624 | 		if err := ctx.Err(); err != nil {
625 | 			return err
626 | 		}
627 | 
628 | 		// can't always trust path names
629 | 		file.NameInArchive = path.Clean(file.NameInArchive)
630 | 
631 | 		// avoid infinite walk; apparently, creating a tar file in the target
632 | 		// directory may result in an entry called "." in the archive; see #384
633 | 		if file.NameInArchive == "." {
634 | 			return nil
635 | 		}
636 | 
637 | 		// if the name being requested isn't a directory, return an error similar to
638 | 		// what most OSes return from the readdir system call when given a non-dir
639 | 		if file.NameInArchive == name && !file.IsDir() {
640 | 			return &fs.PathError{Op: "readdir", Path: name, Err: errors.New("not a directory")}
641 | 		}
642 | 
643 | 		// index this file info for quick access
644 | 		f.contents[file.NameInArchive] = file
645 | 
646 | 		// amortize the DirEntry list per directory, and prefer the real entry's DirEntry over an implicit/fake
647 | 		// one we may have created earlier; first try to find if it exists, and if so, replace the value;
648 | 		// otherwise insert it in sorted position
649 | 		dir := path.Dir(file.NameInArchive)
650 | 		dirEntry := fs.FileInfoToDirEntry(file)
651 | 		idx, found := slices.BinarySearchFunc(f.dirs[dir], dirEntry, func(a, b fs.DirEntry) int {
652 | 			return strings.Compare(a.Name(), b.Name())
653 | 		})
654 | 		if found {
655 | 			f.dirs[dir][idx] = dirEntry
656 | 		} else {
657 | 			f.dirs[dir] = slices.Insert(f.dirs[dir], idx, dirEntry)
658 | 		}
659 | 
660 | 		// this loop looks like an abomination, but it's really quite simple: we're
661 | 		// just iterating the directories of the path up to the root; i.e. we lob off
662 | 		// the base (last component) of the path until no separators remain, i.e. only
663 | 		// one component remains -- then loop again to make sure it's not a duplicate
664 | 		// (start without the base, since we know the full filename is an actual entry
665 | 		// in the archive, we don't need to create an implicit directory entry for it)
666 | 		startingPath := path.Dir(file.NameInArchive)
667 | 		for dir, base := path.Dir(startingPath), path.Base(startingPath); base != "."; dir, base = path.Dir(dir), path.Base(dir) {
668 | 			if err := ctx.Err(); err != nil {
669 | 				return err
670 | 			}
671 | 
672 | 			var dirInfo fs.DirEntry = implicitDirInfo{implicitDirEntry{base}}
673 | 
674 | 			// we are "filling in" any directories that could potentially be only implicit,
675 | 			// and since a nested directory can have more than 1 item, we need to prevent
676 | 			// duplication; for example: given a/b/c and a/b/d, we need to avoid adding
677 | 			// an entry for "b" twice within "a" -- hence we search for it first, and if
678 | 			// it doesn't already exist, we insert it in sorted position
679 | 			idx, found := slices.BinarySearchFunc(f.dirs[dir], dirInfo, func(a, b fs.DirEntry) int {
680 | 				return strings.Compare(a.Name(), b.Name())
681 | 			})
682 | 			if !found {
683 | 				f.dirs[dir] = slices.Insert(f.dirs[dir], idx, dirInfo)
684 | 			}
685 | 		}
686 | 
687 | 		return nil
688 | 	}
689 | 
690 | 	var inputStream io.Reader = archiveFile
691 | 	if f.Stream != nil {
692 | 		inputStream = io.NewSectionReader(f.Stream, 0, f.Stream.Size())
693 | 	}
694 | 
695 | 	err = f.Format.Extract(f.context(), inputStream, handler)
696 | 	if err != nil {
697 | 		// these being non-nil implies that we have indexed the archive,
698 | 		// but if an error occurred, we likely only got part of the way
699 | 		// through and our index is incomplete, and we'd have to re-walk
700 | 		// the whole thing anyway; so reset these to nil to avoid bugs
701 | 		f.dirs = nil
702 | 		f.contents = nil
703 | 		return nil, fmt.Errorf("extract: %w", err)
704 | 	}
705 | 
706 | 	return f.dirs[name], nil
707 | }
708 | 
709 | // Sub returns an FS corresponding to the subtree rooted at dir.
710 | func (f *ArchiveFS) Sub(dir string) (fs.FS, error) {
711 | 	if !fs.ValidPath(dir) {
712 | 		return nil, &fs.PathError{Op: "sub", Path: dir, Err: fs.ErrInvalid}
713 | 	}
714 | 	info, err := f.Stat(dir)
715 | 	if err != nil {
716 | 		return nil, err
717 | 	}
718 | 	if !info.IsDir() {
719 | 		return nil, fmt.Errorf("%s is not a directory", dir)
720 | 	}
721 | 	// result is the same as what we're starting with, except
722 | 	// we indicate a path prefix to be used for all operations;
723 | 	// the reason we don't append to the Path field directly
724 | 	// is because the input might be a stream rather than a
725 | 	// path on disk, and the Prefix field is applied on both
726 | 	result := f
727 | 	result.Prefix = dir
728 | 	return result, nil
729 | }
730 | 
731 | // DeepFS is a fs.FS that represents the real file system, but also has
732 | // the ability to traverse into archive files as if they were part of the
733 | // regular file system. If a filename component ends with an archive
734 | // extension (e.g. .zip, .tar, .tar.gz, etc.), then the remainder of the
735 | // filepath will be considered to be inside that archive.
736 | //
737 | // This allows treating archive files transparently as if they were part
738 | // of the regular file system during a walk, which can be extremely useful
739 | // for accessing data in an "ordinary" walk of the disk, without needing to
740 | // first extract all the archives and use more disk space.
741 | //
742 | // Archives within archives are not supported.
743 | //
744 | // The listing of archive entries is retained for the lifetime of the
745 | // DeepFS value for efficiency, but this can use more memory if archives
746 | // contain a lot of files.
747 | //
748 | // The exported fields may be changed during the lifetime of a DeepFS value
749 | // (but not concurrently). It is safe to use this type as an FS concurrently.
750 | type DeepFS struct {
751 | 	// The root filepath using OS separator, even if it
752 | 	// traverses into an archive.
753 | 	Root string
754 | 
755 | 	// An optional context, mainly for cancellation.
756 | 	Context context.Context
757 | 
758 | 	// remember archive file systems for efficiency
759 | 	inners map[string]fs.FS
760 | 	mu     sync.Mutex
761 | }
762 | 
763 | func (fsys *DeepFS) Open(name string) (fs.File, error) {
764 | 	if !fs.ValidPath(name) {
765 | 		return nil, &fs.PathError{Op: "open", Path: name, Err: fmt.Errorf("%w: %s", fs.ErrInvalid, name)}
766 | 	}
767 | 	name = path.Join(filepath.ToSlash(fsys.Root), name)
768 | 	realPath, innerPath := fsys.SplitPath(name)
769 | 	if innerPath != "" {
770 | 		if innerFsys := fsys.getInnerFsys(realPath); innerFsys != nil {
771 | 			return innerFsys.Open(innerPath)
772 | 		}
773 | 	}
774 | 	return os.Open(realPath)
775 | }
776 | 
777 | func (fsys *DeepFS) Stat(name string) (fs.FileInfo, error) {
778 | 	if !fs.ValidPath(name) {
779 | 		return nil, &fs.PathError{Op: "stat", Path: name, Err: fmt.Errorf("%w: %s", fs.ErrInvalid, name)}
780 | 	}
781 | 	name = path.Join(filepath.ToSlash(fsys.Root), name)
782 | 	realPath, innerPath := fsys.SplitPath(name)
783 | 	if innerPath != "" {
784 | 		if innerFsys := fsys.getInnerFsys(realPath); innerFsys != nil {
785 | 			return fs.Stat(innerFsys, innerPath)
786 | 		}
787 | 	}
788 | 	return os.Stat(realPath)
789 | }
790 | 
791 | // ReadDir returns the directory listing for the given directory name,
792 | // but for any entries that appear by their file extension to be archive
793 | // files, they are slightly modified to always return true for IsDir(),
794 | // since we have the unique ability to list the contents of archives as
795 | // if they were directories.
796 | func (fsys *DeepFS) ReadDir(name string) ([]fs.DirEntry, error) {
797 | 	if !fs.ValidPath(name) {
798 | 		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fmt.Errorf("%w: %s", fs.ErrInvalid, name)}
799 | 	}
800 | 	name = path.Join(filepath.ToSlash(fsys.Root), name)
801 | 	realPath, innerPath := fsys.SplitPath(name)
802 | 	if innerPath != "" {
803 | 		if innerFsys := fsys.getInnerFsys(realPath); innerFsys != nil {
804 | 			return fs.ReadDir(innerFsys, innerPath)
805 | 		}
806 | 	}
807 | 	entries, err := os.ReadDir(realPath)
808 | 	if err != nil {
809 | 		return nil, err
810 | 	}
811 | 	// make sure entries that appear to be archive files indicate they are a directory
812 | 	// so the fs package will try to walk them
813 | 	for i, entry := range entries {
814 | 		if PathIsArchive(entry.Name()) {
815 | 			entries[i] = alwaysDirEntry{entry}
816 | 		}
817 | 	}
818 | 	return entries, nil
819 | }
820 | 
821 | // getInnerFsys reuses "inner" file systems, because for example, archives.ArchiveFS
822 | // amortizes directory entries with the first call to ReadDir; if we don't reuse the
823 | // file systems then they have to rescan the same archive multiple times.
824 | func (fsys *DeepFS) getInnerFsys(realPath string) fs.FS {
825 | 	realPath = filepath.Clean(realPath)
826 | 
827 | 	fsys.mu.Lock()
828 | 	defer fsys.mu.Unlock()
829 | 
830 | 	if fsys.inners == nil {
831 | 		fsys.inners = make(map[string]fs.FS)
832 | 	} else if innerFsys, ok := fsys.inners[realPath]; ok {
833 | 		return innerFsys
834 | 	}
835 | 	innerFsys, err := FileSystem(fsys.context(), realPath, nil)
836 | 	if err == nil {
837 | 		fsys.inners[realPath] = innerFsys
838 | 		return innerFsys
839 | 	}
840 | 	return nil
841 | }
842 | 
843 | // SplitPath splits a file path into the "real" path and the "inner" path components,
844 | // where the split point is the first extension of an archive filetype like ".zip" or
845 | // ".tar.gz" that occurs in the path.
846 | //
847 | // The real path is the path that can be accessed on disk and will be returned with
848 | // platform filepath separators. The inner path is the io/fs-compatible path that can
849 | // be used within the archive.
850 | //
851 | // If no archive extension is found in the path, only the realPath is returned.
852 | // If the input path is precisely an archive file (i.e. ends with an archive file
853 | // extension), then innerPath is returned as "." which indicates the root of the archive.
854 | func (*DeepFS) SplitPath(path string) (realPath, innerPath string) {
855 | 	if len(path) < 2 {
856 | 		realPath = path
857 | 		return
858 | 	}
859 | 
860 | 	// slightly more LoC, but more efficient, than exploding the path on every slash,
861 | 	// is segmenting the path by using indices and looking at slices of the same
862 | 	// string on every iteration; this avoids many allocations which can be valuable
863 | 	// since this can be a hot path
864 | 
865 | 	// start at 1 instead of 0 because we know if the first slash is at 0, the part will be empty
866 | 	start, end := 1, strings.Index(path[1:], "/")+1
867 | 	if end-start < 0 {
868 | 		end = len(path)
869 | 	}
870 | 
871 | 	for {
872 | 		part := strings.TrimRight(strings.ToLower(path[start:end]), " ")
873 | 		if PathIsArchive(part) {
874 | 			// we've found an archive extension, so the path until the end of this segment is
875 | 			// the "real" OS path, and what remains (if anything( is the path within the archive
876 | 			realPath = filepath.Clean(filepath.FromSlash(path[:end]))
877 | 
878 | 			if end < len(path) {
879 | 				innerPath = path[end+1:]
880 | 			} else {
881 | 				// signal to the caller that this is an archive,
882 | 				// even though it is the very root of the archive
883 | 				innerPath = "."
884 | 			}
885 | 			return
886 | 
887 | 		}
888 | 
889 | 		// advance to the next segment, or end of string
890 | 		start = end + 1
891 | 		if start > len(path) {
892 | 			break
893 | 		}
894 | 		end = strings.Index(path[start:], "/") + start
895 | 		if end-start < 0 {
896 | 			end = len(path)
897 | 		}
898 | 	}
899 | 
900 | 	// no archive extension found, so entire path is real path
901 | 	realPath = filepath.Clean(filepath.FromSlash(path))
902 | 	return
903 | }
904 | 
905 | func (fsys *DeepFS) context() context.Context {
906 | 	if fsys.Context != nil {
907 | 		return fsys.Context
908 | 	}
909 | 	return context.Background()
910 | }
911 | 
912 | // alwaysDirEntry always returns true for IsDir(). Because
913 | // DeepFS is able to walk archive files as directories,
914 | // this is used to trick fs.WalkDir to think they are
915 | // directories and thus traverse into them.
916 | type alwaysDirEntry struct {
917 | 	fs.DirEntry
918 | }
919 | 
920 | func (alwaysDirEntry) IsDir() bool { return true }
921 | 
922 | // archiveExtensions contains extensions for popular and supported
923 | // archive types; sorted by popularity and with respect to some
924 | // being prefixed by other extensions.
925 | var archiveExtensions = []string{
926 | 	".zip",
927 | 	".tar",
928 | 	".tgz",
929 | 	".tar.gz",
930 | 	".tar.bz2",
931 | 	".tar.zst",
932 | 	".tar.lz4",
933 | 	".tar.xz",
934 | 	".tar.sz",
935 | 	".tar.s2",
936 | 	".tar.lz",
937 | }
938 | 
939 | // PathIsArchive returns true if the path ends with an archive file (i.e.
940 | // whether the path traverse to an archive) solely by lexical analysis (no
941 | // reading the files or headers is performed).
942 | func PathIsArchive(path string) bool {
943 | 	// normalize the extension
944 | 	path = strings.ToLower(path)
945 | 	for _, ext := range archiveExtensions {
946 | 		// Check the full ext
947 | 		if strings.HasSuffix(path, ext) {
948 | 			return true
949 | 		}
950 | 	}
951 | 
952 | 	return false
953 | }
954 | 
955 | // PathContainsArchive returns true if the path contains an archive file (i.e.
956 | // whether the path traverses into an archive) solely by lexical analysis (no
957 | // reading of files or headers is performed). Such a path is not typically
958 | // usable by the OS, but can be used by the DeepFS type. Slash must be the
959 | // path component separator. Example: "/foo/example.zip/path/in/archive"
960 | func PathContainsArchive(path string) bool {
961 | 	pathPlusSep := path + "/"
962 | 	for _, ext := range archiveExtensions {
963 | 		if strings.Contains(pathPlusSep, ext+"/") {
964 | 			return true
965 | 		}
966 | 	}
967 | 	return false
968 | }
969 | 
970 | // TopDirOpen is a special Open() function that may be useful if
971 | // a file system root was created by extracting an archive.
972 | //
973 | // It first tries the file name as given, but if that returns an
974 | // error, it tries the name without the first element of the path.
975 | // In other words, if "a/b/c" returns an error, then "b/c" will
976 | // be tried instead.
977 | //
978 | // Consider an archive that contains a file "a/b/c". When the
979 | // archive is extracted, the contents may be created without a
980 | // new parent/root folder to contain them, and the path of the
981 | // same file outside the archive may be lacking an exclusive root
982 | // or parent container. Thus it is likely for a file system
983 | // created for the same files extracted to disk to be rooted at
984 | // one of the top-level files/folders from the archive instead of
985 | // a parent folder. For example, the file known as "a/b/c" when
986 | // rooted at the archive becomes "b/c" after extraction when rooted
987 | // at "a" on disk (because no new, exclusive top-level folder was
988 | // created). This difference in paths can make it difficult to use
989 | // archives and directories uniformly. Hence these TopDir* functions
990 | // which attempt to smooth over the difference.
991 | //
992 | // Some extraction utilities do create a container folder for
993 | // archive contents when extracting, in which case the user
994 | // may give that path as the root. In that case, these TopDir*
995 | // functions are not necessary (but aren't harmful either). They
996 | // are primarily useful if you are not sure whether the root is
997 | // an archive file or is an extracted archive file, as they will
998 | // work with the same filename/path inputs regardless of the
999 | // presence of a top-level directory.
1000 | func TopDirOpen(fsys fs.FS, name string) (fs.File, error) {
1001 | 	file, err := fsys.Open(name)
1002 | 	if err == nil {
1003 | 		return file, nil
1004 | 	}
1005 | 	return fsys.Open(pathWithoutTopDir(name))
1006 | }
1007 | 
1008 | // TopDirStat is like TopDirOpen but for Stat.
1009 | func TopDirStat(fsys fs.FS, name string) (fs.FileInfo, error) {
1010 | 	info, err := fs.Stat(fsys, name)
1011 | 	if err == nil {
1012 | 		return info, nil
1013 | 	}
1014 | 	return fs.Stat(fsys, pathWithoutTopDir(name))
1015 | }
1016 | 
1017 | // TopDirReadDir is like TopDirOpen but for ReadDir.
1018 | func TopDirReadDir(fsys fs.FS, name string) ([]fs.DirEntry, error) {
1019 | 	entries, err := fs.ReadDir(fsys, name)
1020 | 	if err == nil {
1021 | 		return entries, nil
1022 | 	}
1023 | 	return fs.ReadDir(fsys, pathWithoutTopDir(name))
1024 | }
1025 | 
1026 | func pathWithoutTopDir(fpath string) string {
1027 | 	slashIdx := strings.Index(fpath, "/")
1028 | 	if slashIdx < 0 {
1029 | 		return fpath
1030 | 	}
1031 | 	return fpath[slashIdx+1:]
1032 | }
1033 | 
1034 | // dirFile implements the fs.ReadDirFile interface.
1035 | type dirFile struct {
1036 | 	info        fs.FileInfo
1037 | 	entries     []fs.DirEntry
1038 | 	entriesRead int // used for paging with ReadDir(n)
1039 | }
1040 | 
1041 | func (dirFile) Read([]byte) (int, error)      { return 0, errors.New("cannot read a directory file") }
1042 | func (df dirFile) Stat() (fs.FileInfo, error) { return df.info, nil }
1043 | func (dirFile) Close() error                  { return nil }
1044 | 
1045 | // ReadDir implements [fs.ReadDirFile].
1046 | func (df *dirFile) ReadDir(n int) ([]fs.DirEntry, error) {
1047 | 	if n <= 0 {
1048 | 		return df.entries, nil
1049 | 	}
1050 | 	if df.entriesRead >= len(df.entries) {
1051 | 		return nil, io.EOF
1052 | 	}
1053 | 	if df.entriesRead+n > len(df.entries) {
1054 | 		n = len(df.entries) - df.entriesRead
1055 | 	}
1056 | 	entries := df.entries[df.entriesRead : df.entriesRead+n]
1057 | 	df.entriesRead += n
1058 | 	return entries, nil
1059 | }
1060 | 
1061 | // dirFileInfo is an implementation of fs.FileInfo that
1062 | // is only used for files that are directories. It always
1063 | // returns 0 size, directory bit set in the mode, and
1064 | // true for IsDir. It is often used as the FileInfo for
1065 | // dirFile values.
1066 | type dirFileInfo struct {
1067 | 	fs.FileInfo
1068 | }
1069 | 
1070 | func (dirFileInfo) Size() int64            { return 0 }
1071 | func (info dirFileInfo) Mode() fs.FileMode { return info.FileInfo.Mode() | fs.ModeDir }
1072 | func (dirFileInfo) IsDir() bool            { return true }
1073 | 
1074 | // fileInArchive represents a file that is opened from within an archive.
1075 | // It implements fs.File.
1076 | type fileInArchive struct {
1077 | 	io.ReadCloser
1078 | 	info fs.FileInfo
1079 | }
1080 | 
1081 | func (af fileInArchive) Stat() (fs.FileInfo, error) { return af.info, nil }
1082 | 
1083 | // closeBoth closes both the file and an associated
1084 | // closer, such as a (de)compressor that wraps the
1085 | // reading/writing of the file. See issue #365. If a
1086 | // better solution is found, I'd probably prefer that.
1087 | type closeBoth struct {
1088 | 	fs.File
1089 | 	c io.Closer // usually the archive or the decompressor
1090 | }
1091 | 
1092 | // Close closes both the file and the associated closer. It always calls
1093 | // Close() on both, but if multiple errors occur they are wrapped together.
1094 | func (dc closeBoth) Close() error {
1095 | 	var err error
1096 | 	if dc.File != nil {
1097 | 		if err2 := dc.File.Close(); err2 != nil {
1098 | 			err = fmt.Errorf("closing file: %w", err2)
1099 | 		}
1100 | 	}
1101 | 	if dc.c != nil {
1102 | 		if err2 := dc.c.Close(); err2 != nil {
1103 | 			if err == nil {
1104 | 				err = fmt.Errorf("closing closer: %w", err2)
1105 | 			} else {
1106 | 				err = fmt.Errorf("%w; additionally, closing closer: %w", err, err2)
1107 | 			}
1108 | 		}
1109 | 	}
1110 | 	return err
1111 | }
1112 | 
1113 | // implicitDirEntry represents a directory that does
1114 | // not actually exist in the archive but is inferred
1115 | // from the paths of actual files in the archive.
1116 | type implicitDirEntry struct{ name string }
1117 | 
1118 | func (e implicitDirEntry) Name() string    { return e.name }
1119 | func (implicitDirEntry) IsDir() bool       { return true }
1120 | func (implicitDirEntry) Type() fs.FileMode { return fs.ModeDir }
1121 | func (e implicitDirEntry) Info() (fs.FileInfo, error) {
1122 | 	return implicitDirInfo{e}, nil
1123 | }
1124 | 
1125 | // implicitDirInfo is a fs.FileInfo for an implicit directory
1126 | // (implicitDirEntry) value. This is used when an archive may
1127 | // not contain actual entries for a directory, but we need to
1128 | // pretend it exists so its contents can be discovered and
1129 | // traversed.
1130 | type implicitDirInfo struct{ implicitDirEntry }
1131 | 
1132 | func (d implicitDirInfo) Name() string      { return d.name }
1133 | func (implicitDirInfo) Size() int64         { return 0 }
1134 | func (d implicitDirInfo) Mode() fs.FileMode { return d.Type() }
1135 | func (implicitDirInfo) ModTime() time.Time  { return time.Time{} }
1136 | func (implicitDirInfo) Sys() any            { return nil }
1137 | 
1138 | // dotFileInfo is a fs.FileInfo that can be used to provide
1139 | // the true name instead of ".".
1140 | type dotFileInfo struct {
1141 | 	fs.FileInfo
1142 | 	name string
1143 | }
1144 | 
1145 | func (d dotFileInfo) Name() string { return d.name }
1146 | 
1147 | // Interface guards
1148 | var (
1149 | 	_ fs.ReadDirFS = (*FileFS)(nil)
1150 | 	_ fs.StatFS    = (*FileFS)(nil)
1151 | 
1152 | 	_ fs.ReadDirFS = (*ArchiveFS)(nil)
1153 | 	_ fs.StatFS    = (*ArchiveFS)(nil)
1154 | 	_ fs.SubFS     = (*ArchiveFS)(nil)
1155 | )
```

fs_test.go
```
1 | package archives
2 | 
3 | import (
4 | 	"bytes"
5 | 	"context"
6 | 	_ "embed"
7 | 	"fmt"
8 | 	"io"
9 | 	"io/fs"
10 | 	"log"
11 | 	"net/http"
12 | 	"os"
13 | 	"path"
14 | 	"path/filepath"
15 | 	"reflect"
16 | 	"sort"
17 | 	"testing"
18 | )
19 | 
20 | func TestPathWithoutTopDir(t *testing.T) {
21 | 	for i, tc := range []struct {
22 | 		input, expect string
23 | 	}{
24 | 		{
25 | 			input:  "a/b/c",
26 | 			expect: "b/c",
27 | 		},
28 | 		{
29 | 			input:  "b/c",
30 | 			expect: "c",
31 | 		},
32 | 		{
33 | 			input:  "c",
34 | 			expect: "c",
35 | 		},
36 | 		{
37 | 			input:  "",
38 | 			expect: "",
39 | 		},
40 | 	} {
41 | 		if actual := pathWithoutTopDir(tc.input); actual != tc.expect {
42 | 			t.Errorf("Test %d (input=%s): Expected '%s' but got '%s'", i, tc.input, tc.expect, actual)
43 | 		}
44 | 	}
45 | }
46 | 
47 | func TestSplitPath(t *testing.T) {
48 | 	d := DeepFS{}
49 | 	for i, testCase := range []struct {
50 | 		input, expectedReal, expectedInner string
51 | 	}{
52 | 		{
53 | 			input:         "/",
54 | 			expectedReal:  "/",
55 | 			expectedInner: "",
56 | 		},
57 | 		{
58 | 			input:         "foo",
59 | 			expectedReal:  "foo",
60 | 			expectedInner: "",
61 | 		},
62 | 		{
63 | 			input:         "foo/bar",
64 | 			expectedReal:  filepath.Join("foo", "bar"),
65 | 			expectedInner: "",
66 | 		},
67 | 		{
68 | 			input:         "foo.zip",
69 | 			expectedReal:  filepath.Join("foo.zip"),
70 | 			expectedInner: ".",
71 | 		},
72 | 		{
73 | 			input:         "foo.zip/a",
74 | 			expectedReal:  "foo.zip",
75 | 			expectedInner: "a",
76 | 		},
77 | 		{
78 | 			input:         "foo.zip/a/b",
79 | 			expectedReal:  "foo.zip",
80 | 			expectedInner: "a/b",
81 | 		},
82 | 		{
83 | 			input:         "a/b/foobar.zip/c",
84 | 			expectedReal:  filepath.Join("a", "b", "foobar.zip"),
85 | 			expectedInner: "c",
86 | 		},
87 | 		{
88 | 			input:         "a/foo.zip/b/test.tar",
89 | 			expectedReal:  filepath.Join("a", "foo.zip"),
90 | 			expectedInner: "b/test.tar",
91 | 		},
92 | 		{
93 | 			input:         "a/foo.zip/b/test.tar/c",
94 | 			expectedReal:  filepath.Join("a", "foo.zip"),
95 | 			expectedInner: "b/test.tar/c",
96 | 		},
97 | 	} {
98 | 		actualReal, actualInner := d.SplitPath(testCase.input)
99 | 		if actualReal != testCase.expectedReal {
100 | 			t.Errorf("Test %d (input=%q): expected real path %q but got %q", i, testCase.input, testCase.expectedReal, actualReal)
101 | 		}
102 | 		if actualInner != testCase.expectedInner {
103 | 			t.Errorf("Test %d (input=%q): expected inner path %q but got %q", i, testCase.input, testCase.expectedInner, actualInner)
104 | 		}
105 | 	}
106 | }
107 | 
108 | func TestPathContainsArchive(t *testing.T) {
109 | 	for i, testCase := range []struct {
110 | 		input    string
111 | 		expected bool
112 | 	}{
113 | 		{
114 | 			input:    "",
115 | 			expected: false,
116 | 		},
117 | 		{
118 | 			input:    "foo",
119 | 			expected: false,
120 | 		},
121 | 		{
122 | 			input:    "foo.zip",
123 | 			expected: true,
124 | 		},
125 | 		{
126 | 			input:    "a/b/c.tar.gz",
127 | 			expected: true,
128 | 		},
129 | 		{
130 | 			input:    "a/b/c.tar.gz/d",
131 | 			expected: true,
132 | 		},
133 | 		{
134 | 			input:    "a/b/c.txt",
135 | 			expected: false,
136 | 		},
137 | 	} {
138 | 		actual := PathContainsArchive(testCase.input)
139 | 		if actual != testCase.expected {
140 | 			t.Errorf("Test %d (input=%q): expected %v but got %v", i, testCase.input, testCase.expected, actual)
141 | 		}
142 | 	}
143 | }
144 | 
145 | var (
146 | 	//go:embed testdata/test.zip
147 | 	testZIP []byte
148 | 	//go:embed testdata/unordered.zip
149 | 	unorderZip []byte
150 | )
151 | 
152 | func TestSelfTar(t *testing.T) {
153 | 	fn := "testdata/self-tar.tar"
154 | 	fh, err := os.Open(fn)
155 | 	if err != nil {
156 | 		t.Errorf("Could not load test tar: %v", fn)
157 | 	}
158 | 	fstat, err := os.Stat(fn)
159 | 	if err != nil {
160 | 		t.Errorf("Could not stat test tar: %v", fn)
161 | 	}
162 | 	fsys := &ArchiveFS{
163 | 		Stream: io.NewSectionReader(fh, 0, fstat.Size()),
164 | 		Format: Tar{},
165 | 	}
166 | 	var count int
167 | 	err = fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
168 | 		if count > 10 {
169 | 			t.Error("walking test tar appears to be recursing in error")
170 | 			return fmt.Errorf("recursing tar: %v", fn)
171 | 		}
172 | 		count++
173 | 		return nil
174 | 	})
175 | 	if err != nil {
176 | 		t.Error(err)
177 | 	}
178 | }
179 | 
180 | func ExampleArchiveFS_Stream() {
181 | 	fsys := &ArchiveFS{
182 | 		Stream: io.NewSectionReader(bytes.NewReader(testZIP), 0, int64(len(testZIP))),
183 | 		Format: Zip{},
184 | 	}
185 | 	// You can serve the contents in a web server:
186 | 	http.Handle("/static", http.StripPrefix("/static",
187 | 		http.FileServer(http.FS(fsys))))
188 | 
189 | 	// Or read the files using fs functions:
190 | 	dis, err := fsys.ReadDir(".")
191 | 	if err != nil {
192 | 		log.Fatal(err)
193 | 	}
194 | 	for _, di := range dis {
195 | 		fmt.Println(di.Name())
196 | 		b, err := fs.ReadFile(fsys, path.Join(".", di.Name()))
197 | 		if err != nil {
198 | 			log.Fatal(err)
199 | 		}
200 | 		fmt.Println(bytes.Contains(b, []byte("granted")))
201 | 	}
202 | 	// Output:
203 | 	// LICENSE
204 | 	// true
205 | }
206 | 
207 | func TestArchiveFS_ReadDir(t *testing.T) {
208 | 	for _, tc := range []struct {
209 | 		name    string
210 | 		archive ArchiveFS
211 | 		want    map[string][]string
212 | 	}{
213 | 		{
214 | 			name: "test.zip",
215 | 			archive: ArchiveFS{
216 | 				Stream: io.NewSectionReader(bytes.NewReader(testZIP), 0, int64(len(testZIP))),
217 | 				Format: Zip{},
218 | 			},
219 | 			// unzip -l testdata/test.zip
220 | 			want: map[string][]string{
221 | 				".": {"LICENSE"},
222 | 			},
223 | 		},
224 | 		{
225 | 			name: "unordered.zip",
226 | 			archive: ArchiveFS{
227 | 				Stream: io.NewSectionReader(bytes.NewReader(unorderZip), 0, int64(len(unorderZip))),
228 | 				Format: Zip{},
229 | 			},
230 | 			// unzip -l testdata/unordered.zip, note entry 1/1 and 1/2 are separated by contents of directory 2
231 | 			want: map[string][]string{
232 | 				".": {"1", "2"},
233 | 				"1": {"1", "2"},
234 | 				"2": {"1"},
235 | 			},
236 | 		},
237 | 	} {
238 | 		tc := tc
239 | 		t.Run(tc.name, func(t *testing.T) {
240 | 			t.Parallel()
241 | 			fsys := tc.archive
242 | 			for baseDir, wantLS := range tc.want {
243 | 				t.Run(fmt.Sprintf("ReadDir(%q)", baseDir), func(t *testing.T) {
244 | 					dis, err := fsys.ReadDir(baseDir)
245 | 					if err != nil {
246 | 						t.Error(err)
247 | 					}
248 | 
249 | 					dirs := []string{}
250 | 					for _, di := range dis {
251 | 						dirs = append(dirs, di.Name())
252 | 					}
253 | 
254 | 					// Stabilize the sort order
255 | 					sort.Strings(dirs)
256 | 
257 | 					if !reflect.DeepEqual(wantLS, dirs) {
258 | 						t.Errorf("ReadDir() got: %v, want: %v", dirs, wantLS)
259 | 					}
260 | 				})
261 | 
262 | 				// Uncomment to reproduce https://github.com/mholt/archiver/issues/340.
263 | 				t.Run(fmt.Sprintf("Open(%s)", baseDir), func(t *testing.T) {
264 | 					f, err := fsys.Open(baseDir)
265 | 					if err != nil {
266 | 						t.Errorf("fsys.Open(%q): %#v %s", baseDir, err, err)
267 | 						return
268 | 					}
269 | 
270 | 					rdf, ok := f.(fs.ReadDirFile)
271 | 					if !ok {
272 | 						t.Errorf("fsys.Open(%q) did not return a fs.ReadDirFile, got: %#v", baseDir, f)
273 | 					}
274 | 
275 | 					dis, err := rdf.ReadDir(-1)
276 | 					if err != nil {
277 | 						t.Error(err)
278 | 					}
279 | 
280 | 					dirs := []string{}
281 | 					for _, di := range dis {
282 | 						dirs = append(dirs, di.Name())
283 | 					}
284 | 
285 | 					// Stabilize the sort order
286 | 					sort.Strings(dirs)
287 | 
288 | 					if !reflect.DeepEqual(wantLS, dirs) {
289 | 						t.Errorf("Open().ReadDir(-1) got: %v, want: %v", dirs, wantLS)
290 | 					}
291 | 				})
292 | 			}
293 | 		})
294 | 	}
295 | }
296 | 
297 | func TestFileSystem(t *testing.T) {
298 | 	ctx := context.Background()
299 | 	filename := "testdata/test.zip"
300 | 
301 | 	checkFS := func(t *testing.T, fsys fs.FS) {
302 | 		license, err := fsys.Open("LICENSE")
303 | 		if err != nil {
304 | 			t.Fatal(err)
305 | 		}
306 | 		b, err := io.ReadAll(license)
307 | 		if err != nil {
308 | 			t.Fatal(err)
309 | 		}
310 | 		if len(b) == 0 {
311 | 			t.Fatal("empty file")
312 | 		}
313 | 		err = license.Close()
314 | 		if err != nil {
315 | 			t.Fatal(err)
316 | 		}
317 | 	}
318 | 
319 | 	t.Run("filename", func(t *testing.T) {
320 | 		fsys, err := FileSystem(ctx, filename, nil)
321 | 		if err != nil {
322 | 			t.Fatal(err)
323 | 		}
324 | 		checkFS(t, fsys)
325 | 	})
326 | 
327 | 	t.Run("stream", func(t *testing.T) {
328 | 		f, err := os.Open(filename)
329 | 		if err != nil {
330 | 			t.Fatal(err)
331 | 		}
332 | 		t.Cleanup(func() {
333 | 			err = f.Close()
334 | 			if err != nil {
335 | 				t.Error(err)
336 | 			}
337 | 		})
338 | 		fsys, err := FileSystem(ctx, "", f)
339 | 		if err != nil {
340 | 			t.Fatal(err)
341 | 		}
342 | 		checkFS(t, fsys)
343 | 	})
344 | 
345 | 	t.Run("filename and stream", func(t *testing.T) {
346 | 		f, err := os.Open(filename)
347 | 		if err != nil {
348 | 			t.Fatal(err)
349 | 		}
350 | 		t.Cleanup(func() {
351 | 			err = f.Close()
352 | 			if err != nil {
353 | 				t.Error(err)
354 | 			}
355 | 		})
356 | 		fsys, err := FileSystem(ctx, "test.zip", f)
357 | 		if err != nil {
358 | 			t.Fatal(err)
359 | 		}
360 | 		checkFS(t, fsys)
361 | 	})
362 | }
```

go.mod
```
1 | module github.com/mholt/archives
2 | 
3 | go 1.22.2
4 | 
5 | toolchain go1.23.2
6 | 
7 | require (
8 | 	github.com/andybalholm/brotli v1.1.2-0.20250424173009-453214e765f3
9 | 	github.com/dsnet/compress v0.0.2-0.20230904184137-39efe44ab707
10 | 	github.com/klauspost/compress v1.17.11
11 | 	github.com/klauspost/pgzip v1.2.6
12 | 	github.com/nwaples/rardecode/v2 v2.1.0
13 | 	github.com/therootcompany/xz v1.0.1
14 | 	github.com/ulikunitz/xz v0.5.12
15 | )
16 | 
17 | require (
18 | 	github.com/STARRY-S/zip v0.2.1
19 | 	github.com/bodgit/sevenzip v1.6.0
20 | 	github.com/minio/minlz v1.0.0
21 | 	github.com/pierrec/lz4/v4 v4.1.21
22 | 	github.com/sorairolake/lzip-go v0.3.5
23 | 	golang.org/x/text v0.20.0
24 | )
25 | 
26 | require (
27 | 	github.com/bodgit/plumbing v1.3.0 // indirect
28 | 	github.com/bodgit/windows v1.0.1 // indirect
29 | 	github.com/hashicorp/errwrap v1.1.0 // indirect
30 | 	github.com/hashicorp/go-multierror v1.1.1 // indirect
31 | 	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
32 | 	go4.org v0.0.0-20230225012048-214862532bf5 // indirect
33 | )
```

go.sum
```
1 | cloud.google.com/go v0.26.0/go.mod h1:aQUYkXzVsufM+DwF1aE+0xfcU+56JwCaLick0ClmMTw=
2 | cloud.google.com/go v0.34.0/go.mod h1:aQUYkXzVsufM+DwF1aE+0xfcU+56JwCaLick0ClmMTw=
3 | cloud.google.com/go v0.38.0/go.mod h1:990N+gfupTy94rShfmMCWGDn0LpTmnzTp2qbd1dvSRU=
4 | cloud.google.com/go v0.44.1/go.mod h1:iSa0KzasP4Uvy3f1mN/7PiObzGgflwredwwASm/v6AU=
5 | cloud.google.com/go v0.44.2/go.mod h1:60680Gw3Yr4ikxnPRS/oxxkBccT6SA1yMk63TGekxKY=
6 | cloud.google.com/go v0.45.1/go.mod h1:RpBamKRgapWJb87xiFSdk4g1CME7QZg3uwTez+TSTjc=
7 | cloud.google.com/go v0.46.3/go.mod h1:a6bKKbmY7er1mI7TEI4lsAkts/mkhTSZK8w33B4RAg0=
8 | cloud.google.com/go v0.50.0/go.mod h1:r9sluTvynVuxRIOHXQEHMFffphuXHOMZMycpNR5e6To=
9 | cloud.google.com/go v0.53.0/go.mod h1:fp/UouUEsRkN6ryDKNW/Upv/JBKnv6WDthjR6+vze6M=
10 | cloud.google.com/go/bigquery v1.0.1/go.mod h1:i/xbL2UlR5RvWAURpBYZTtm/cXjCha9lbfbpx4poX+o=
11 | cloud.google.com/go/bigquery v1.3.0/go.mod h1:PjpwJnslEMmckchkHFfq+HTD2DmtT67aNFKH1/VBDHE=
12 | cloud.google.com/go/datastore v1.0.0/go.mod h1:LXYbyblFSglQ5pkeyhO+Qmw7ukd3C+pD7TKLgZqpHYE=
13 | cloud.google.com/go/pubsub v1.0.1/go.mod h1:R0Gpsv3s54REJCy4fxDixWD93lHJMoZTyQ2kNxGRt3I=
14 | cloud.google.com/go/pubsub v1.1.0/go.mod h1:EwwdRX2sKPjnvnqCa270oGRyludottCI76h+R3AArQw=
15 | cloud.google.com/go/storage v1.0.0/go.mod h1:IhtSnM/ZTZV8YYJWCY8RULGVqBDmpoyjwiyrjsg+URw=
16 | cloud.google.com/go/storage v1.5.0/go.mod h1:tpKbwo567HUNpVclU5sGELwQWBDZ8gh0ZeosJ0Rtdos=
17 | dmitri.shuralyov.com/gpu/mtl v0.0.0-20190408044501-666a987793e9/go.mod h1:H6x//7gZCb22OMCxBHrMx7a5I7Hp++hsVxbQ4BYO7hU=
18 | github.com/BurntSushi/toml v0.3.1/go.mod h1:xHWCNGjB5oqiDr8zfno3MHue2Ht5sIBksp03qcyfWMU=
19 | github.com/BurntSushi/xgb v0.0.0-20160522181843-27f122750802/go.mod h1:IVnqGOEym/WlBOVXweHU+Q+/VP0lqqI8lqeDx9IjBqo=
20 | github.com/STARRY-S/zip v0.2.1 h1:pWBd4tuSGm3wtpoqRZZ2EAwOmcHK6XFf7bU9qcJXyFg=
21 | github.com/STARRY-S/zip v0.2.1/go.mod h1:xNvshLODWtC4EJ702g7cTYn13G53o1+X9BWnPFpcWV4=
22 | github.com/andybalholm/brotli v1.1.1 h1:PR2pgnyFznKEugtsUo0xLdDop5SKXd5Qf5ysW+7XdTA=
23 | github.com/andybalholm/brotli v1.1.1/go.mod h1:05ib4cKhjx3OQYUY22hTVd34Bc8upXjOLL2rKwwZBoA=
24 | github.com/andybalholm/brotli v1.1.2-0.20250424173009-453214e765f3 h1:8PmGpDEZl9yDpcdEr6Odf23feCxK3LNUNMxjXg41pZQ=
25 | github.com/andybalholm/brotli v1.1.2-0.20250424173009-453214e765f3/go.mod h1:05ib4cKhjx3OQYUY22hTVd34Bc8upXjOLL2rKwwZBoA=
26 | github.com/bodgit/plumbing v1.3.0 h1:pf9Itz1JOQgn7vEOE7v7nlEfBykYqvUYioC61TwWCFU=
27 | github.com/bodgit/plumbing v1.3.0/go.mod h1:JOTb4XiRu5xfnmdnDJo6GmSbSbtSyufrsyZFByMtKEs=
28 | github.com/bodgit/sevenzip v1.6.0 h1:a4R0Wu6/P1o1pP/3VV++aEOcyeBxeO/xE2Y9NSTrr6A=
29 | github.com/bodgit/sevenzip v1.6.0/go.mod h1:zOBh9nJUof7tcrlqJFv1koWRrhz3LbDbUNngkuZxLMc=
30 | github.com/bodgit/windows v1.0.1 h1:tF7K6KOluPYygXa3Z2594zxlkbKPAOvqr97etrGNIz4=
31 | github.com/bodgit/windows v1.0.1/go.mod h1:a6JLwrB4KrTR5hBpp8FI9/9W9jJfeQ2h4XDXU74ZCdM=
32 | github.com/census-instrumentation/opencensus-proto v0.2.1/go.mod h1:f6KPmirojxKA12rnyqOA5BBL4O983OfeGPqjHWSTneU=
33 | github.com/chzyer/logex v1.1.10/go.mod h1:+Ywpsq7O8HXn0nuIou7OrIPyXbp3wmkHB+jjWRnGsAI=
34 | github.com/chzyer/readline v0.0.0-20180603132655-2972be24d48e/go.mod h1:nSuG5e5PlCu98SY8svDHJxuZscDgtXS6KTTbou5AhLI=
35 | github.com/chzyer/test v0.0.0-20180213035817-a1ea475d72b1/go.mod h1:Q3SI9o4m/ZMnBNeIyt5eFwwo7qiLfzFZmjNmxjkiQlU=
36 | github.com/client9/misspell v0.3.4/go.mod h1:qj6jICC3Q7zFZvVWo7KLAzC3yx5G7kyvSDkc90ppPyw=
37 | github.com/davecgh/go-spew v1.1.0/go.mod h1:J7Y8YcW2NihsgmVo/mv3lAwl/skON4iLHjSsI+c5H38=
38 | github.com/davecgh/go-spew v1.1.1 h1:vj9j/u1bqnvCEfJOwUhtlOARqs3+rkHYY13jYWTU97c=
39 | github.com/davecgh/go-spew v1.1.1/go.mod h1:J7Y8YcW2NihsgmVo/mv3lAwl/skON4iLHjSsI+c5H38=
40 | github.com/dsnet/compress v0.0.2-0.20230904184137-39efe44ab707 h1:2tV76y6Q9BB+NEBasnqvs7e49aEBFI8ejC89PSnWH+4=
41 | github.com/dsnet/compress v0.0.2-0.20230904184137-39efe44ab707/go.mod h1:qssHWj60/X5sZFNxpG4HBPDHVqxNm4DfnCKgrbZOT+s=
42 | github.com/dsnet/golib v0.0.0-20171103203638-1ea166775780/go.mod h1:Lj+Z9rebOhdfkVLjJ8T6VcRQv3SXugXy999NBtR9aFY=
43 | github.com/envoyproxy/go-control-plane v0.9.1-0.20191026205805-5f8ba28d4473/go.mod h1:YTl/9mNaCwkRvm6d1a2C3ymFceY/DCBVvsKhRF0iEA4=
44 | github.com/envoyproxy/protoc-gen-validate v0.1.0/go.mod h1:iSmxcyjqTsJpI2R4NaDN7+kN2VEUnK/pcBlmesArF7c=
45 | github.com/go-gl/glfw v0.0.0-20190409004039-e6da0acd62b1/go.mod h1:vR7hzQXu2zJy9AVAgeJqvqgH9Q5CA+iKCZ2gyEVpxRU=
46 | github.com/go-gl/glfw/v3.3/glfw v0.0.0-20191125211704-12ad95a8df72/go.mod h1:tQ2UAYgL5IevRw8kRxooKSPJfGvJ9fJQFa0TUsXzTg8=
47 | github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b/go.mod h1:SBH7ygxi8pfUlaOkMMuAQtPIUF8ecWP5IEl/CR7VP2Q=
48 | github.com/golang/groupcache v0.0.0-20190702054246-869f871628b6/go.mod h1:cIg4eruTrX1D+g88fzRXU5OdNfaM+9IcxsU14FzY7Hc=
49 | github.com/golang/groupcache v0.0.0-20191227052852-215e87163ea7/go.mod h1:cIg4eruTrX1D+g88fzRXU5OdNfaM+9IcxsU14FzY7Hc=
50 | github.com/golang/groupcache v0.0.0-20200121045136-8c9f03a8e57e/go.mod h1:cIg4eruTrX1D+g88fzRXU5OdNfaM+9IcxsU14FzY7Hc=
51 | github.com/golang/mock v1.1.1/go.mod h1:oTYuIxOrZwtPieC+H1uAHpcLFnEyAGVDL/k47Jfbm0A=
52 | github.com/golang/mock v1.2.0/go.mod h1:oTYuIxOrZwtPieC+H1uAHpcLFnEyAGVDL/k47Jfbm0A=
53 | github.com/golang/mock v1.3.1/go.mod h1:sBzyDLLjw3U8JLTeZvSv8jJB+tU5PVekmnlKIyFUx0Y=
54 | github.com/golang/mock v1.4.0/go.mod h1:UOMv5ysSaYNkG+OFQykRIcU/QvvxJf3p21QfJ2Bt3cw=
55 | github.com/golang/protobuf v1.2.0/go.mod h1:6lQm79b+lXiMfvg/cZm0SGofjICqVBUtrP5yJMmIC1U=
56 | github.com/golang/protobuf v1.3.1/go.mod h1:6lQm79b+lXiMfvg/cZm0SGofjICqVBUtrP5yJMmIC1U=
57 | github.com/golang/protobuf v1.3.2/go.mod h1:6lQm79b+lXiMfvg/cZm0SGofjICqVBUtrP5yJMmIC1U=
58 | github.com/golang/protobuf v1.3.3/go.mod h1:vzj43D7+SQXF/4pzW/hwtAqwc6iTitCiVSaWz5lYuqw=
59 | github.com/google/btree v0.0.0-20180813153112-4030bb1f1f0c/go.mod h1:lNA+9X1NB3Zf8V7Ke586lFgjr2dZNuvo3lPJSGZ5JPQ=
60 | github.com/google/btree v1.0.0/go.mod h1:lNA+9X1NB3Zf8V7Ke586lFgjr2dZNuvo3lPJSGZ5JPQ=
61 | github.com/google/go-cmp v0.2.0/go.mod h1:oXzfMopK8JAjlY9xF4vHSVASa0yLyX7SntLO5aqRK0M=
62 | github.com/google/go-cmp v0.3.0/go.mod h1:8QqcDgzrUqlUb/G2PQTWiueGozuR1884gddMywk6iLU=
63 | github.com/google/go-cmp v0.3.1/go.mod h1:8QqcDgzrUqlUb/G2PQTWiueGozuR1884gddMywk6iLU=
64 | github.com/google/go-cmp v0.4.0/go.mod h1:v8dTdLbMG2kIc/vJvl+f65V22dbkXbowE6jgT/gNBxE=
65 | github.com/google/go-cmp v0.5.5/go.mod h1:v8dTdLbMG2kIc/vJvl+f65V22dbkXbowE6jgT/gNBxE=
66 | github.com/google/martian v2.1.0+incompatible/go.mod h1:9I4somxYTbIHy5NJKHRl3wXiIaQGbYVAs8BPL6v8lEs=
67 | github.com/google/pprof v0.0.0-20181206194817-3ea8567a2e57/go.mod h1:zfwlbNMJ+OItoe0UupaVj+oy1omPYYDuagoSzA8v9mc=
68 | github.com/google/pprof v0.0.0-20190515194954-54271f7e092f/go.mod h1:zfwlbNMJ+OItoe0UupaVj+oy1omPYYDuagoSzA8v9mc=
69 | github.com/google/pprof v0.0.0-20200212024743-f11f1df84d12/go.mod h1:ZgVRPoUq/hfqzAqh7sHMqb3I9Rq5C59dIz2SbBwJ4eM=
70 | github.com/google/renameio v0.1.0/go.mod h1:KWCgfxg9yswjAJkECMjeO8J8rahYeXnNhOm40UhjYkI=
71 | github.com/googleapis/gax-go/v2 v2.0.4/go.mod h1:0Wqv26UfaUD9n4G6kQubkQ+KchISgw+vpHVxEJEs9eg=
72 | github.com/googleapis/gax-go/v2 v2.0.5/go.mod h1:DWXyrwAJ9X0FpwwEdw+IPEYBICEFu5mhpdKc/us6bOk=
73 | github.com/hashicorp/errwrap v1.0.0/go.mod h1:YH+1FKiLXxHSkmPseP+kNlulaMuP3n2brvKWEqk/Jc4=
74 | github.com/hashicorp/errwrap v1.1.0 h1:OxrOeh75EUXMY8TBjag2fzXGZ40LB6IKw45YeGUDY2I=
75 | github.com/hashicorp/errwrap v1.1.0/go.mod h1:YH+1FKiLXxHSkmPseP+kNlulaMuP3n2brvKWEqk/Jc4=
76 | github.com/hashicorp/go-multierror v1.1.1 h1:H5DkEtf6CXdFp0N0Em5UCwQpXMWke8IA0+lD48awMYo=
77 | github.com/hashicorp/go-multierror v1.1.1/go.mod h1:iw975J/qwKPdAO1clOe2L8331t/9/fmwbPZ6JB6eMoM=
78 | github.com/hashicorp/golang-lru v0.5.0/go.mod h1:/m3WP610KZHVQ1SGc6re/UDhFvYD7pJ4Ao+sR/qLZy8=
79 | github.com/hashicorp/golang-lru v0.5.1/go.mod h1:/m3WP610KZHVQ1SGc6re/UDhFvYD7pJ4Ao+sR/qLZy8=
80 | github.com/hashicorp/golang-lru/v2 v2.0.7 h1:a+bsQ5rvGLjzHuww6tVxozPZFVghXaHOwFs4luLUK2k=
81 | github.com/hashicorp/golang-lru/v2 v2.0.7/go.mod h1:QeFd9opnmA6QUJc5vARoKUSoFhyfM2/ZepoAG6RGpeM=
82 | github.com/ianlancetaylor/demangle v0.0.0-20181102032728-5e5cf60278f6/go.mod h1:aSSvb/t6k1mPoxDqO4vJh6VOCGPwU4O0C2/Eqndh1Sc=
83 | github.com/jstemmer/go-junit-report v0.0.0-20190106144839-af01ea7f8024/go.mod h1:6v2b51hI/fHJwM22ozAgKL4VKDeJcHhJFhtBdhmNjmU=
84 | github.com/jstemmer/go-junit-report v0.9.1/go.mod h1:Brl9GWCQeLvo8nXZwPNNblvFj/XSXhF0NWZEnDohbsk=
85 | github.com/kisielk/gotool v1.0.0/go.mod h1:XhKaO+MFFWcvkIS/tQcRk01m1F5IRFswLeQ+oQHNcck=
86 | github.com/klauspost/compress v1.4.1/go.mod h1:RyIbtBH6LamlWaDj8nUwkbUhJ87Yi3uG0guNDohfE1A=
87 | github.com/klauspost/compress v1.17.11 h1:In6xLpyWOi1+C7tXUUWv2ot1QvBjxevKAaI6IXrJmUc=
88 | github.com/klauspost/compress v1.17.11/go.mod h1:pMDklpSncoRMuLFrf1W9Ss9KT+0rH90U12bZKk7uwG0=
89 | github.com/klauspost/cpuid v1.2.0/go.mod h1:Pj4uuM528wm8OyEC2QMXAi2YiTZ96dNQPGgoMS4s3ek=
90 | github.com/klauspost/pgzip v1.2.6 h1:8RXeL5crjEUFnR2/Sn6GJNWtSQ3Dk8pq4CL3jvdDyjU=
91 | github.com/klauspost/pgzip v1.2.6/go.mod h1:Ch1tH69qFZu15pkjo5kYi6mth2Zzwzt50oCQKQE9RUs=
92 | github.com/kr/pretty v0.1.0/go.mod h1:dAy3ld7l9f0ibDNOQOHHMYYIIbhfbHSm3C4ZsoJORNo=
93 | github.com/kr/pty v1.1.1/go.mod h1:pFQYn66WHrOpPYNljwOMqo10TkYh1fy3cYio2l3bCsQ=
94 | github.com/kr/text v0.1.0/go.mod h1:4Jbv+DJW3UT/LiOwJeYQe1efqtUx/iVham/4vfdArNI=
95 | github.com/minio/minlz v1.0.0 h1:Kj7aJZ1//LlTP1DM8Jm7lNKvvJS2m74gyyXXn3+uJWQ=
96 | github.com/minio/minlz v1.0.0/go.mod h1:qT0aEB35q79LLornSzeDH75LBf3aH1MV+jB5w9Wasec=
97 | github.com/nwaples/rardecode/v2 v2.1.0 h1:JQl9ZoBPDy+nIZGb1mx8+anfHp/LV3NE2MjMiv0ct/U=
98 | github.com/nwaples/rardecode/v2 v2.1.0/go.mod h1:7uz379lSxPe6j9nvzxUZ+n7mnJNgjsRNb6IbvGVHRmw=
99 | github.com/pierrec/lz4/v4 v4.1.21 h1:yOVMLb6qSIDP67pl/5F7RepeKYu/VmTyEXvuMI5d9mQ=
100 | github.com/pierrec/lz4/v4 v4.1.21/go.mod h1:gZWDp/Ze/IJXGXf23ltt2EXimqmTUXEy0GFuRQyBid4=
101 | github.com/pmezard/go-difflib v1.0.0 h1:4DBwDE0NGyQoBHbLQYPwSUPoCMWR5BEzIk/f1lZbAQM=
102 | github.com/pmezard/go-difflib v1.0.0/go.mod h1:iKH77koFhYxTK1pcRnkKkqfTogsbg7gZNVY4sRDYZ/4=
103 | github.com/prometheus/client_model v0.0.0-20190812154241-14fe0d1b01d4/go.mod h1:xMI15A0UPsDsEKsMN9yxemIoYk6Tm2C1GtYGdfGttqA=
104 | github.com/rogpeppe/go-internal v1.3.0/go.mod h1:M8bDsm7K2OlrFYOpmOWEs/qY81heoFRclV5y23lUDJ4=
105 | github.com/rwcarlsen/goexif v0.0.0-20190401172101-9e8deecbddbd/go.mod h1:hPqNNc0+uJM6H+SuU8sEs5K5IQeKccPqeSjfgcKGgPk=
106 | github.com/sorairolake/lzip-go v0.3.5 h1:ms5Xri9o1JBIWvOFAorYtUNik6HI3HgBTkISiqu0Cwg=
107 | github.com/sorairolake/lzip-go v0.3.5/go.mod h1:N0KYq5iWrMXI0ZEXKXaS9hCyOjZUQdBDEIbXfoUwbdk=
108 | github.com/stretchr/objx v0.1.0/go.mod h1:HFkY916IF+rwdDfMAkV7OtwuqBVzrE8GR6GFx+wExME=
109 | github.com/stretchr/objx v0.4.0/go.mod h1:YvHI0jy2hoMjB+UWwv71VJQ9isScKT/TqJzVSSt89Yw=
110 | github.com/stretchr/objx v0.5.0/go.mod h1:Yh+to48EsGEfYuaHDzXPcE3xhTkx73EhmCGUpEOglKo=
111 | github.com/stretchr/testify v1.4.0/go.mod h1:j7eGeouHqKxXV5pUuKE4zz7dFj8WfuZ+81PSLYec5m4=
112 | github.com/stretchr/testify v1.7.1/go.mod h1:6Fq8oRcR53rry900zMqJjRRixrwX3KX962/h/Wwjteg=
113 | github.com/stretchr/testify v1.8.0/go.mod h1:yNjHg4UonilssWZ8iaSj1OCr/vHnekPRkoO+kdMU+MU=
114 | github.com/stretchr/testify v1.8.1/go.mod h1:w2LPCIKwWwSfY2zedu0+kehJoqGctiVI29o6fzry7u4=
115 | github.com/stretchr/testify v1.9.0 h1:HtqpIVDClZ4nwg75+f6Lvsy/wHu+3BoSGCbBAcpTsTg=
116 | github.com/stretchr/testify v1.9.0/go.mod h1:r2ic/lqez/lEtzL7wO/rwa5dbSLXVDPFyf8C91i36aY=
117 | github.com/therootcompany/xz v1.0.1 h1:CmOtsn1CbtmyYiusbfmhmkpAAETj0wBIH6kCYaX+xzw=
118 | github.com/therootcompany/xz v1.0.1/go.mod h1:3K3UH1yCKgBneZYhuQUvJ9HPD19UEXEI0BWbMn8qNMY=
119 | github.com/ulikunitz/xz v0.5.8/go.mod h1:nbz6k7qbPmH4IRqmfOplQw/tblSgqTqBwxkY0oWt/14=
120 | github.com/ulikunitz/xz v0.5.12 h1:37Nm15o69RwBkXM0J6A5OlE67RZTfzUxTj8fB3dfcsc=
121 | github.com/ulikunitz/xz v0.5.12/go.mod h1:nbz6k7qbPmH4IRqmfOplQw/tblSgqTqBwxkY0oWt/14=
122 | github.com/xyproto/randomstring v1.0.5 h1:YtlWPoRdgMu3NZtP45drfy1GKoojuR7hmRcnhZqKjWU=
123 | github.com/xyproto/randomstring v1.0.5/go.mod h1:rgmS5DeNXLivK7YprL0pY+lTuhNQW3iGxZ18UQApw/E=
124 | github.com/yuin/goldmark v1.4.13/go.mod h1:6yULJ656Px+3vBD8DxQVa3kxgyrAnzto9xy5taEt/CY=
125 | go.opencensus.io v0.21.0/go.mod h1:mSImk1erAIZhrmZN+AvHh14ztQfjbGwt4TtuofqLduU=
126 | go.opencensus.io v0.22.0/go.mod h1:+kGneAE2xo2IficOXnaByMWTGM9T73dGwxeWcUqIpI8=
127 | go.opencensus.io v0.22.2/go.mod h1:yxeiOL68Rb0Xd1ddK5vPZ/oVn4vY4Ynel7k9FzqtOIw=
128 | go.opencensus.io v0.22.3/go.mod h1:yxeiOL68Rb0Xd1ddK5vPZ/oVn4vY4Ynel7k9FzqtOIw=
129 | go4.org v0.0.0-20230225012048-214862532bf5 h1:nifaUDeh+rPaBCMPMQHZmvJf+QdpLFnuQPwx+LxVmtc=
130 | go4.org v0.0.0-20230225012048-214862532bf5/go.mod h1:F57wTi5Lrj6WLyswp5EYV1ncrEbFGHD4hhz6S1ZYeaU=
131 | golang.org/x/crypto v0.0.0-20190308221718-c2843e01d9a2/go.mod h1:djNgcEr1/C05ACkg1iLfiJU5Ep61QUkGW8qpdssI0+w=
132 | golang.org/x/crypto v0.0.0-20190510104115-cbcb75029529/go.mod h1:yigFU9vqHzYiE8UmvKecakEJjdnWj3jj499lnFckfCI=
133 | golang.org/x/crypto v0.0.0-20190605123033-f99c8df09eb5/go.mod h1:yigFU9vqHzYiE8UmvKecakEJjdnWj3jj499lnFckfCI=
134 | golang.org/x/crypto v0.0.0-20191011191535-87dc89f01550/go.mod h1:yigFU9vqHzYiE8UmvKecakEJjdnWj3jj499lnFckfCI=
135 | golang.org/x/crypto v0.0.0-20210921155107-089bfa567519/go.mod h1:GvvjBRRGRdwPK5ydBHafDWAxML/pGHZbMvKqRZ5+Abc=
136 | golang.org/x/exp v0.0.0-20190121172915-509febef88a4/go.mod h1:CJ0aWSM057203Lf6IL+f9T1iT9GByDxfZKAQTCR3kQA=
137 | golang.org/x/exp v0.0.0-20190306152737-a1d7652674e8/go.mod h1:CJ0aWSM057203Lf6IL+f9T1iT9GByDxfZKAQTCR3kQA=
138 | golang.org/x/exp v0.0.0-20190510132918-efd6b22b2522/go.mod h1:ZjyILWgesfNpC6sMxTJOJm9Kp84zZh5NQWvqDGG3Qr8=
139 | golang.org/x/exp v0.0.0-20190829153037-c13cbed26979/go.mod h1:86+5VVa7VpoJ4kLfm080zCjGlMRFzhUhsZKEZO7MGek=
140 | golang.org/x/exp v0.0.0-20191030013958-a1ab85dbe136/go.mod h1:JXzH8nQsPlswgeRAPE3MuO9GYsAcnJvJ4vnMwN/5qkY=
141 | golang.org/x/exp v0.0.0-20191129062945-2f5052295587/go.mod h1:2RIsYlXP63K8oxa1u096TMicItID8zy7Y6sNkU49FU4=
142 | golang.org/x/exp v0.0.0-20191227195350-da58074b4299/go.mod h1:2RIsYlXP63K8oxa1u096TMicItID8zy7Y6sNkU49FU4=
143 | golang.org/x/exp v0.0.0-20200207192155-f17229e696bd/go.mod h1:J/WKrq2StrnmMY6+EHIKF9dgMWnmCNThgcyBT1FY9mM=
144 | golang.org/x/image v0.0.0-20190227222117-0694c2d4d067/go.mod h1:kZ7UVZpmo3dzQBMxlp+ypCbDeSB+sBbTgSJuh5dn5js=
145 | golang.org/x/image v0.0.0-20190802002840-cff245a6509b/go.mod h1:FeLwcggjj3mMvU+oOTbSwawSJRM1uh48EjtB4UJZlP0=
146 | golang.org/x/lint v0.0.0-20181026193005-c67002cb31c3/go.mod h1:UVdnD1Gm6xHRNCYTkRU2/jEulfH38KcIWyp/GAMgvoE=
147 | golang.org/x/lint v0.0.0-20190227174305-5b3e6a55c961/go.mod h1:wehouNa3lNwaWXcvxsM5YxQ5yQlVC4a0KAMCusXpPoU=
148 | golang.org/x/lint v0.0.0-20190301231843-5614ed5bae6f/go.mod h1:UVdnD1Gm6xHRNCYTkRU2/jEulfH38KcIWyp/GAMgvoE=
149 | golang.org/x/lint v0.0.0-20190313153728-d0100b6bd8b3/go.mod h1:6SW0HCj/g11FgYtHlgUYUwCkIfeOF89ocIRzGO/8vkc=
150 | golang.org/x/lint v0.0.0-20190409202823-959b441ac422/go.mod h1:6SW0HCj/g11FgYtHlgUYUwCkIfeOF89ocIRzGO/8vkc=
151 | golang.org/x/lint v0.0.0-20190909230951-414d861bb4ac/go.mod h1:6SW0HCj/g11FgYtHlgUYUwCkIfeOF89ocIRzGO/8vkc=
152 | golang.org/x/lint v0.0.0-20190930215403-16217165b5de/go.mod h1:6SW0HCj/g11FgYtHlgUYUwCkIfeOF89ocIRzGO/8vkc=
153 | golang.org/x/lint v0.0.0-20191125180803-fdd1cda4f05f/go.mod h1:5qLYkcX4OjUUV8bRuDixDT3tpyyb+LUpUlRWLxfhWrs=
154 | golang.org/x/lint v0.0.0-20200130185559-910be7a94367/go.mod h1:3xt1FjdF8hUf6vQPIChWIBhFzV8gjjsPE/fR3IyQdNY=
155 | golang.org/x/mobile v0.0.0-20190312151609-d3739f865fa6/go.mod h1:z+o9i4GpDbdi3rU15maQ/Ox0txvL9dWGYEHz965HBQE=
156 | golang.org/x/mobile v0.0.0-20190719004257-d2bd2a29d028/go.mod h1:E/iHnbuqvinMTCcRqshq8CkpyQDoeVncDDYHnLhea+o=
157 | golang.org/x/mod v0.0.0-20190513183733-4bf6d317e70e/go.mod h1:mXi4GBBbnImb6dmsKGUJ2LatrhH/nqhxcFungHvyanc=
158 | golang.org/x/mod v0.1.0/go.mod h1:0QHyrYULN0/3qlju5TqG8bIK38QM8yzMo5ekMj3DlcY=
159 | golang.org/x/mod v0.1.1-0.20191105210325-c90efee705ee/go.mod h1:QqPTAvyqsEbceGzBzNggFXnrqF1CaUcvgkdR5Ot7KZg=
160 | golang.org/x/mod v0.2.0/go.mod h1:s0Qsj1ACt9ePp/hMypM3fl4fZqREWJwdYDEqhRiZZUA=
161 | golang.org/x/mod v0.6.0-dev.0.20220419223038-86c51ed26bb4/go.mod h1:jJ57K6gSWd91VN4djpZkiMVwK6gcyfeH4XE8wZrZaV4=
162 | golang.org/x/net v0.0.0-20180724234803-3673e40ba225/go.mod h1:mL1N/T3taQHkDXs73rZJwtUhF3w3ftmwwsq0BUmARs4=
163 | golang.org/x/net v0.0.0-20180826012351-8a410e7b638d/go.mod h1:mL1N/T3taQHkDXs73rZJwtUhF3w3ftmwwsq0BUmARs4=
164 | golang.org/x/net v0.0.0-20190108225652-1e06a53dbb7e/go.mod h1:mL1N/T3taQHkDXs73rZJwtUhF3w3ftmwwsq0BUmARs4=
165 | golang.org/x/net v0.0.0-20190213061140-3a22650c66bd/go.mod h1:mL1N/T3taQHkDXs73rZJwtUhF3w3ftmwwsq0BUmARs4=
166 | golang.org/x/net v0.0.0-20190311183353-d8887717615a/go.mod h1:t9HGtf8HONx5eT2rtn7q6eTqICYqUVnKs3thJo3Qplg=
167 | golang.org/x/net v0.0.0-20190404232315-eb5bcb51f2a3/go.mod h1:t9HGtf8HONx5eT2rtn7q6eTqICYqUVnKs3thJo3Qplg=
168 | golang.org/x/net v0.0.0-20190501004415-9ce7a6920f09/go.mod h1:t9HGtf8HONx5eT2rtn7q6eTqICYqUVnKs3thJo3Qplg=
169 | golang.org/x/net v0.0.0-20190503192946-f4e77d36d62c/go.mod h1:t9HGtf8HONx5eT2rtn7q6eTqICYqUVnKs3thJo3Qplg=
170 | golang.org/x/net v0.0.0-20190603091049-60506f45cf65/go.mod h1:HSz+uSET+XFnRR8LxR5pz3Of3rY3CfYBVs4xY44aLks=
171 | golang.org/x/net v0.0.0-20190620200207-3b0461eec859/go.mod h1:z5CRVTTTmAJ677TzLLGU+0bjPO0LkuOLi4/5GtJWs/s=
172 | golang.org/x/net v0.0.0-20190724013045-ca1201d0de80/go.mod h1:z5CRVTTTmAJ677TzLLGU+0bjPO0LkuOLi4/5GtJWs/s=
173 | golang.org/x/net v0.0.0-20191209160850-c0dbc17a3553/go.mod h1:z5CRVTTTmAJ677TzLLGU+0bjPO0LkuOLi4/5GtJWs/s=
174 | golang.org/x/net v0.0.0-20200202094626-16171245cfb2/go.mod h1:z5CRVTTTmAJ677TzLLGU+0bjPO0LkuOLi4/5GtJWs/s=
175 | golang.org/x/net v0.0.0-20210226172049-e18ecbb05110/go.mod h1:m0MpNAwzfU5UDzcl9v0D8zg8gWTRqZa9RBIspLL5mdg=
176 | golang.org/x/net v0.0.0-20220722155237-a158d28d115b/go.mod h1:XRhObCWvk6IyKnWLug+ECip1KBveYUHfp+8e9klMJ9c=
177 | golang.org/x/net v0.7.0/go.mod h1:2Tu9+aMcznHK/AK1HMvgo6xiTLG5rD5rZLDS+rp2Bjs=
178 | golang.org/x/oauth2 v0.0.0-20180821212333-d2e6202438be/go.mod h1:N/0e6XlmueqKjAGxoOufVs8QHGRruUQn6yWY3a++T0U=
179 | golang.org/x/oauth2 v0.0.0-20190226205417-e64efc72b421/go.mod h1:gOpvHmFTYa4IltrdGE7lF6nIHvwfUNPOp7c8zoXwtLw=
180 | golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45/go.mod h1:gOpvHmFTYa4IltrdGE7lF6nIHvwfUNPOp7c8zoXwtLw=
181 | golang.org/x/oauth2 v0.0.0-20191202225959-858c2ad4c8b6/go.mod h1:gOpvHmFTYa4IltrdGE7lF6nIHvwfUNPOp7c8zoXwtLw=
182 | golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d/go.mod h1:gOpvHmFTYa4IltrdGE7lF6nIHvwfUNPOp7c8zoXwtLw=
183 | golang.org/x/sync v0.0.0-20180314180146-1d60e4601c6f/go.mod h1:RxMgew5VJxzue5/jJTE5uejpjVlOe/izrB70Jof72aM=
184 | golang.org/x/sync v0.0.0-20181108010431-42b317875d0f/go.mod h1:RxMgew5VJxzue5/jJTE5uejpjVlOe/izrB70Jof72aM=
185 | golang.org/x/sync v0.0.0-20181221193216-37e7f081c4d4/go.mod h1:RxMgew5VJxzue5/jJTE5uejpjVlOe/izrB70Jof72aM=
186 | golang.org/x/sync v0.0.0-20190227155943-e225da77a7e6/go.mod h1:RxMgew5VJxzue5/jJTE5uejpjVlOe/izrB70Jof72aM=
187 | golang.org/x/sync v0.0.0-20190423024810-112230192c58/go.mod h1:RxMgew5VJxzue5/jJTE5uejpjVlOe/izrB70Jof72aM=
188 | golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e/go.mod h1:RxMgew5VJxzue5/jJTE5uejpjVlOe/izrB70Jof72aM=
189 | golang.org/x/sync v0.0.0-20220722155255-886fb9371eb4/go.mod h1:RxMgew5VJxzue5/jJTE5uejpjVlOe/izrB70Jof72aM=
190 | golang.org/x/sync v0.9.0 h1:fEo0HyrW1GIgZdpbhCRO0PkJajUS5H9IFUztCgEo2jQ=
191 | golang.org/x/sync v0.9.0/go.mod h1:Czt+wKu1gCyEFDUtn0jG5QVvpJ6rzVqr5aXyt9drQfk=
192 | golang.org/x/sys v0.0.0-20180830151530-49385e6e1522/go.mod h1:STP8DvDyc/dI5b8T5hshtkjS+E42TnysNCUPdjciGhY=
193 | golang.org/x/sys v0.0.0-20190215142949-d0b11bdaac8a/go.mod h1:STP8DvDyc/dI5b8T5hshtkjS+E42TnysNCUPdjciGhY=
194 | golang.org/x/sys v0.0.0-20190312061237-fead79001313/go.mod h1:h1NjWce9XRLGQEsW7wpKNCjG9DtNlClVuFLEZdDNbEs=
195 | golang.org/x/sys v0.0.0-20190412213103-97732733099d/go.mod h1:h1NjWce9XRLGQEsW7wpKNCjG9DtNlClVuFLEZdDNbEs=
196 | golang.org/x/sys v0.0.0-20190502145724-3ef323f4f1fd/go.mod h1:h1NjWce9XRLGQEsW7wpKNCjG9DtNlClVuFLEZdDNbEs=
197 | golang.org/x/sys v0.0.0-20190507160741-ecd444e8653b/go.mod h1:h1NjWce9XRLGQEsW7wpKNCjG9DtNlClVuFLEZdDNbEs=
198 | golang.org/x/sys v0.0.0-20190606165138-5da285871e9c/go.mod h1:h1NjWce9XRLGQEsW7wpKNCjG9DtNlClVuFLEZdDNbEs=
199 | golang.org/x/sys v0.0.0-20190624142023-c5567b49c5d0/go.mod h1:h1NjWce9XRLGQEsW7wpKNCjG9DtNlClVuFLEZdDNbEs=
200 | golang.org/x/sys v0.0.0-20190726091711-fc99dfbffb4e/go.mod h1:h1NjWce9XRLGQEsW7wpKNCjG9DtNlClVuFLEZdDNbEs=
201 | golang.org/x/sys v0.0.0-20191204072324-ce4227a45e2e/go.mod h1:h1NjWce9XRLGQEsW7wpKNCjG9DtNlClVuFLEZdDNbEs=
202 | golang.org/x/sys v0.0.0-20191228213918-04cbcbbfeed8/go.mod h1:h1NjWce9XRLGQEsW7wpKNCjG9DtNlClVuFLEZdDNbEs=
203 | golang.org/x/sys v0.0.0-20200212091648-12a6c2dcc1e4/go.mod h1:h1NjWce9XRLGQEsW7wpKNCjG9DtNlClVuFLEZdDNbEs=
204 | golang.org/x/sys v0.0.0-20201119102817-f84b799fce68/go.mod h1:h1NjWce9XRLGQEsW7wpKNCjG9DtNlClVuFLEZdDNbEs=
205 | golang.org/x/sys v0.0.0-20210615035016-665e8c7367d1/go.mod h1:oPkhp1MJrh7nUepCBck5+mAzfO9JrbApNNgaTdGDITg=
206 | golang.org/x/sys v0.0.0-20220520151302-bc2c85ada10a/go.mod h1:oPkhp1MJrh7nUepCBck5+mAzfO9JrbApNNgaTdGDITg=
207 | golang.org/x/sys v0.0.0-20220722155257-8c9f86f7a55f/go.mod h1:oPkhp1MJrh7nUepCBck5+mAzfO9JrbApNNgaTdGDITg=
208 | golang.org/x/sys v0.5.0/go.mod h1:oPkhp1MJrh7nUepCBck5+mAzfO9JrbApNNgaTdGDITg=
209 | golang.org/x/term v0.0.0-20201126162022-7de9c90e9dd1/go.mod h1:bj7SfCRtBDWHUb9snDiAeCFNEtKQo2Wmx5Cou7ajbmo=
210 | golang.org/x/term v0.0.0-20210927222741-03fcf44c2211/go.mod h1:jbD1KX2456YbFQfuXm/mYQcufACuNUgVhRMnK/tPxf8=
211 | golang.org/x/term v0.5.0/go.mod h1:jMB1sMXY+tzblOD4FWmEbocvup2/aLOaQEp7JmGp78k=
212 | golang.org/x/text v0.0.0-20170915032832-14c0d48ead0c/go.mod h1:NqM8EUOU14njkJ3fqMW+pc6Ldnwhi/IjpwHt7yyuwOQ=
213 | golang.org/x/text v0.3.0/go.mod h1:NqM8EUOU14njkJ3fqMW+pc6Ldnwhi/IjpwHt7yyuwOQ=
214 | golang.org/x/text v0.3.1-0.20180807135948-17ff2d5776d2/go.mod h1:NqM8EUOU14njkJ3fqMW+pc6Ldnwhi/IjpwHt7yyuwOQ=
215 | golang.org/x/text v0.3.2/go.mod h1:bEr9sfX3Q8Zfm5fL9x+3itogRgK3+ptLWKqgva+5dAk=
216 | golang.org/x/text v0.3.3/go.mod h1:5Zoc/QRtKVWzQhOtBMvqHzDpF6irO9z98xDceosuGiQ=
217 | golang.org/x/text v0.3.7/go.mod h1:u+2+/6zg+i71rQMx5EYifcz6MCKuco9NR6JIITiCfzQ=
218 | golang.org/x/text v0.7.0/go.mod h1:mrYo+phRRbMaCq/xk9113O4dZlRixOauAjOtrjsXDZ8=
219 | golang.org/x/text v0.20.0 h1:gK/Kv2otX8gz+wn7Rmb3vT96ZwuoxnQlY+HlJVj7Qug=
220 | golang.org/x/text v0.20.0/go.mod h1:D4IsuqiFMhST5bX19pQ9ikHC2GsaKyk/oF+pn3ducp4=
221 | golang.org/x/time v0.0.0-20181108054448-85acf8d2951c/go.mod h1:tRJNPiyCQ0inRvYxbN9jk5I+vvW/OXSQhTDSoE431IQ=
222 | golang.org/x/time v0.0.0-20190308202827-9d24e82272b4/go.mod h1:tRJNPiyCQ0inRvYxbN9jk5I+vvW/OXSQhTDSoE431IQ=
223 | golang.org/x/tools v0.0.0-20180917221912-90fa682c2a6e/go.mod h1:n7NCudcB/nEzxVGmLbDWY5pfWTLqBcC2KZ6jyYvM4mQ=
224 | golang.org/x/tools v0.0.0-20190114222345-bf090417da8b/go.mod h1:n7NCudcB/nEzxVGmLbDWY5pfWTLqBcC2KZ6jyYvM4mQ=
225 | golang.org/x/tools v0.0.0-20190226205152-f727befe758c/go.mod h1:9Yl7xja0Znq3iFh3HoIrodX9oNMXvdceNzlUR8zjMvY=
226 | golang.org/x/tools v0.0.0-20190311212946-11955173bddd/go.mod h1:LCzVGOaR6xXOjkQ3onu1FJEFr0SW1gC7cKk1uF8kGRs=
227 | golang.org/x/tools v0.0.0-20190312151545-0bb0c0a6e846/go.mod h1:LCzVGOaR6xXOjkQ3onu1FJEFr0SW1gC7cKk1uF8kGRs=
228 | golang.org/x/tools v0.0.0-20190312170243-e65039ee4138/go.mod h1:LCzVGOaR6xXOjkQ3onu1FJEFr0SW1gC7cKk1uF8kGRs=
229 | golang.org/x/tools v0.0.0-20190425150028-36563e24a262/go.mod h1:RgjU9mgBXZiqYHBnxXauZ1Gv1EHHAz9KjViQ78xBX0Q=
230 | golang.org/x/tools v0.0.0-20190506145303-2d16b83fe98c/go.mod h1:RgjU9mgBXZiqYHBnxXauZ1Gv1EHHAz9KjViQ78xBX0Q=
231 | golang.org/x/tools v0.0.0-20190524140312-2c0ae7006135/go.mod h1:RgjU9mgBXZiqYHBnxXauZ1Gv1EHHAz9KjViQ78xBX0Q=
232 | golang.org/x/tools v0.0.0-20190606124116-d0a3d012864b/go.mod h1:/rFqwRUd4F7ZHNgwSSTFct+R/Kf4OFW1sUzUTQQTgfc=
233 | golang.org/x/tools v0.0.0-20190621195816-6e04913cbbac/go.mod h1:/rFqwRUd4F7ZHNgwSSTFct+R/Kf4OFW1sUzUTQQTgfc=
234 | golang.org/x/tools v0.0.0-20190628153133-6cdbf07be9d0/go.mod h1:/rFqwRUd4F7ZHNgwSSTFct+R/Kf4OFW1sUzUTQQTgfc=
235 | golang.org/x/tools v0.0.0-20190816200558-6889da9d5479/go.mod h1:b+2E5dAYhXwXZwtnZ6UAqBI28+e2cm9otk0dWdXHAEo=
236 | golang.org/x/tools v0.0.0-20190911174233-4f2ddba30aff/go.mod h1:b+2E5dAYhXwXZwtnZ6UAqBI28+e2cm9otk0dWdXHAEo=
237 | golang.org/x/tools v0.0.0-20191012152004-8de300cfc20a/go.mod h1:b+2E5dAYhXwXZwtnZ6UAqBI28+e2cm9otk0dWdXHAEo=
238 | golang.org/x/tools v0.0.0-20191113191852-77e3bb0ad9e7/go.mod h1:b+2E5dAYhXwXZwtnZ6UAqBI28+e2cm9otk0dWdXHAEo=
239 | golang.org/x/tools v0.0.0-20191115202509-3a792d9c32b2/go.mod h1:b+2E5dAYhXwXZwtnZ6UAqBI28+e2cm9otk0dWdXHAEo=
240 | golang.org/x/tools v0.0.0-20191119224855-298f0cb1881e/go.mod h1:b+2E5dAYhXwXZwtnZ6UAqBI28+e2cm9otk0dWdXHAEo=
241 | golang.org/x/tools v0.0.0-20191125144606-a911d9008d1f/go.mod h1:b+2E5dAYhXwXZwtnZ6UAqBI28+e2cm9otk0dWdXHAEo=
242 | golang.org/x/tools v0.0.0-20191216173652-a0e659d51361/go.mod h1:TB2adYChydJhpapKDTa4BR/hXlZSLoq2Wpct/0txZ28=
243 | golang.org/x/tools v0.0.0-20191227053925-7b8e75db28f4/go.mod h1:TB2adYChydJhpapKDTa4BR/hXlZSLoq2Wpct/0txZ28=
244 | golang.org/x/tools v0.0.0-20200130002326-2f3ba24bd6e7/go.mod h1:TB2adYChydJhpapKDTa4BR/hXlZSLoq2Wpct/0txZ28=
245 | golang.org/x/tools v0.0.0-20200207183749-b753a1ba74fa/go.mod h1:TB2adYChydJhpapKDTa4BR/hXlZSLoq2Wpct/0txZ28=
246 | golang.org/x/tools v0.0.0-20200212150539-ea181f53ac56/go.mod h1:TB2adYChydJhpapKDTa4BR/hXlZSLoq2Wpct/0txZ28=
247 | golang.org/x/tools v0.1.12/go.mod h1:hNGJHUnrk76NpqgfD5Aqm5Crs+Hm0VOH/i9J2+nxYbc=
248 | golang.org/x/xerrors v0.0.0-20190717185122-a985d3407aa7/go.mod h1:I/5z698sn9Ka8TeJc9MKroUUfqBBauWjQqLJ2OPfmY0=
249 | golang.org/x/xerrors v0.0.0-20191011141410-1b5146add898/go.mod h1:I/5z698sn9Ka8TeJc9MKroUUfqBBauWjQqLJ2OPfmY0=
250 | golang.org/x/xerrors v0.0.0-20191204190536-9bdfabe68543/go.mod h1:I/5z698sn9Ka8TeJc9MKroUUfqBBauWjQqLJ2OPfmY0=
251 | google.golang.org/api v0.4.0/go.mod h1:8k5glujaEP+g9n7WNsDg8QP6cUVNI86fCNMcbazEtwE=
252 | google.golang.org/api v0.7.0/go.mod h1:WtwebWUNSVBH/HAw79HIFXZNqEvBhG+Ra+ax0hx3E3M=
253 | google.golang.org/api v0.8.0/go.mod h1:o4eAsZoiT+ibD93RtjEohWalFOjRDx6CVaqeizhEnKg=
254 | google.golang.org/api v0.9.0/go.mod h1:o4eAsZoiT+ibD93RtjEohWalFOjRDx6CVaqeizhEnKg=
255 | google.golang.org/api v0.13.0/go.mod h1:iLdEw5Ide6rF15KTC1Kkl0iskquN2gFfn9o9XIsbkAI=
256 | google.golang.org/api v0.14.0/go.mod h1:iLdEw5Ide6rF15KTC1Kkl0iskquN2gFfn9o9XIsbkAI=
257 | google.golang.org/api v0.15.0/go.mod h1:iLdEw5Ide6rF15KTC1Kkl0iskquN2gFfn9o9XIsbkAI=
258 | google.golang.org/api v0.17.0/go.mod h1:BwFmGc8tA3vsd7r/7kR8DY7iEEGSU04BFxCo5jP/sfE=
259 | google.golang.org/appengine v1.1.0/go.mod h1:EbEs0AVv82hx2wNQdGPgUI5lhzA/G0D9YwlJXL52JkM=
260 | google.golang.org/appengine v1.4.0/go.mod h1:xpcJRLb0r/rnEns0DIKYYv+WjYCduHsrkT7/EB5XEv4=
261 | google.golang.org/appengine v1.5.0/go.mod h1:xpcJRLb0r/rnEns0DIKYYv+WjYCduHsrkT7/EB5XEv4=
262 | google.golang.org/appengine v1.6.1/go.mod h1:i06prIuMbXzDqacNJfV5OdTW448YApPu5ww/cMBSeb0=
263 | google.golang.org/appengine v1.6.5/go.mod h1:8WjMMxjGQR8xUklV/ARdw2HLXBOI7O7uCIDZVag1xfc=
264 | google.golang.org/genproto v0.0.0-20180817151627-c66870c02cf8/go.mod h1:JiN7NxoALGmiZfu7CAH4rXhgtRTLTxftemlI0sWmxmc=
265 | google.golang.org/genproto v0.0.0-20190307195333-5fe7a883aa19/go.mod h1:VzzqZJRnGkLBvHegQrXjBqPurQTc5/KpmUdxsrq26oE=
266 | google.golang.org/genproto v0.0.0-20190418145605-e7d98fc518a7/go.mod h1:VzzqZJRnGkLBvHegQrXjBqPurQTc5/KpmUdxsrq26oE=
267 | google.golang.org/genproto v0.0.0-20190425155659-357c62f0e4bb/go.mod h1:VzzqZJRnGkLBvHegQrXjBqPurQTc5/KpmUdxsrq26oE=
268 | google.golang.org/genproto v0.0.0-20190502173448-54afdca5d873/go.mod h1:VzzqZJRnGkLBvHegQrXjBqPurQTc5/KpmUdxsrq26oE=
269 | google.golang.org/genproto v0.0.0-20190801165951-fa694d86fc64/go.mod h1:DMBHOl98Agz4BDEuKkezgsaosCRResVns1a3J2ZsMNc=
270 | google.golang.org/genproto v0.0.0-20190819201941-24fa4b261c55/go.mod h1:DMBHOl98Agz4BDEuKkezgsaosCRResVns1a3J2ZsMNc=
271 | google.golang.org/genproto v0.0.0-20190911173649-1774047e7e51/go.mod h1:IbNlFCBrqXvoKpeg0TB2l7cyZUmoaFKYIwrEpbDKLA8=
272 | google.golang.org/genproto v0.0.0-20191108220845-16a3f7862a1a/go.mod h1:n3cpQtvxv34hfy77yVDNjmbRyujviMdxYliBSkLhpCc=
273 | google.golang.org/genproto v0.0.0-20191115194625-c23dd37a84c9/go.mod h1:n3cpQtvxv34hfy77yVDNjmbRyujviMdxYliBSkLhpCc=
274 | google.golang.org/genproto v0.0.0-20191216164720-4f79533eabd1/go.mod h1:n3cpQtvxv34hfy77yVDNjmbRyujviMdxYliBSkLhpCc=
275 | google.golang.org/genproto v0.0.0-20191230161307-f3c370f40bfb/go.mod h1:n3cpQtvxv34hfy77yVDNjmbRyujviMdxYliBSkLhpCc=
276 | google.golang.org/genproto v0.0.0-20200212174721-66ed5ce911ce/go.mod h1:55QSHmfGQM9UVYDPBsyGGes0y52j32PQ3BqQfXhyH3c=
277 | google.golang.org/grpc v1.19.0/go.mod h1:mqu4LbDTu4XGKhr4mRzUsmM4RtVoemTSY81AxZiDr8c=
278 | google.golang.org/grpc v1.20.1/go.mod h1:10oTOabMzJvdu6/UiuZezV6QK5dSlG84ov/aaiqXj38=
279 | google.golang.org/grpc v1.21.1/go.mod h1:oYelfM1adQP15Ek0mdvEgi9Df8B9CZIaU1084ijfRaM=
280 | google.golang.org/grpc v1.23.0/go.mod h1:Y5yQAOtifL1yxbo5wqy6BxZv8vAUGQwXBOALyacEbxg=
281 | google.golang.org/grpc v1.26.0/go.mod h1:qbnxyOmOxrQa7FizSgH+ReBfzJrCY1pSN7KXBS8abTk=
282 | google.golang.org/grpc v1.27.0/go.mod h1:qbnxyOmOxrQa7FizSgH+ReBfzJrCY1pSN7KXBS8abTk=
283 | google.golang.org/grpc v1.27.1/go.mod h1:qbnxyOmOxrQa7FizSgH+ReBfzJrCY1pSN7KXBS8abTk=
284 | gopkg.in/check.v1 v0.0.0-20161208181325-20d25e280405/go.mod h1:Co6ibVJAznAaIkqp8huTwlJQCZ016jof/cbN4VW5Yz0=
285 | gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127/go.mod h1:Co6ibVJAznAaIkqp8huTwlJQCZ016jof/cbN4VW5Yz0=
286 | gopkg.in/errgo.v2 v2.1.0/go.mod h1:hNsd1EY+bozCKY1Ytp96fpM3vjJbqLJn88ws8XvfDNI=
287 | gopkg.in/yaml.v2 v2.2.2/go.mod h1:hI93XBmqTisBFMUTm0b8Fm+jr3Dg1NNxqwp+5A1VGuI=
288 | gopkg.in/yaml.v3 v3.0.0-20200313102051-9f266ea9e77c/go.mod h1:K4uyk7z7BCEPqu6E+C64Yfv1cQ7kz7rIZviUmN+EgEM=
289 | gopkg.in/yaml.v3 v3.0.1 h1:fxVm/GzAzEWqLHuvctI91KS9hhNmmWOoWu0XTYJS7CA=
290 | gopkg.in/yaml.v3 v3.0.1/go.mod h1:K4uyk7z7BCEPqu6E+C64Yfv1cQ7kz7rIZviUmN+EgEM=
291 | honnef.co/go/tools v0.0.0-20190102054323-c2f93a96b099/go.mod h1:rf3lG4BRIbNafJWhAfAdb/ePZxsR/4RtNHQocxwk9r4=
292 | honnef.co/go/tools v0.0.0-20190106161140-3f1c8253044a/go.mod h1:rf3lG4BRIbNafJWhAfAdb/ePZxsR/4RtNHQocxwk9r4=
293 | honnef.co/go/tools v0.0.0-20190418001031-e561f6794a2a/go.mod h1:rf3lG4BRIbNafJWhAfAdb/ePZxsR/4RtNHQocxwk9r4=
294 | honnef.co/go/tools v0.0.0-20190523083050-ea95bdfd59fc/go.mod h1:rf3lG4BRIbNafJWhAfAdb/ePZxsR/4RtNHQocxwk9r4=
295 | honnef.co/go/tools v0.0.1-2019.2.3/go.mod h1:a3bituU0lyd329TUQxRnasdCoJDkEUEAqEt0JzvZhAg=
296 | rsc.io/binaryregexp v0.2.0/go.mod h1:qTv7/COck+e2FymRvadv62gMdZztPaShugOCi3I+8D8=
297 | rsc.io/quote/v3 v3.1.0/go.mod h1:yEA65RcK8LyAZtP9Kv3t0HmxON59tX3rD+tICJqUlj0=
298 | rsc.io/sampler v1.3.0/go.mod h1:T1hPZKmBbMNahiBKFy5HrXp6adAjACjK9JXDnKaTXpA=
```

gz.go
```
1 | package archives
2 | 
3 | import (
4 | 	"bytes"
5 | 	"context"
6 | 	"io"
7 | 	"strings"
8 | 
9 | 	"github.com/klauspost/compress/gzip"
10 | 	"github.com/klauspost/pgzip"
11 | )
12 | 
13 | func init() {
14 | 	RegisterFormat(Gz{})
15 | }
16 | 
17 | // Gz facilitates gzip compression.
18 | type Gz struct {
19 | 	// Gzip compression level. See https://pkg.go.dev/compress/flate#pkg-constants
20 | 	// for some predefined constants. If 0, DefaultCompression is assumed rather
21 | 	// than no compression.
22 | 	CompressionLevel int
23 | 
24 | 	// DisableMultistream controls whether the reader supports multistream files.
25 | 	// See https://pkg.go.dev/compress/gzip#example-Reader.Multistream
26 | 	DisableMultistream bool
27 | 
28 | 	// Use a fast parallel Gzip implementation. This is only
29 | 	// effective for large streams (about 1 MB or greater).
30 | 	Multithreaded bool
31 | }
32 | 
33 | func (Gz) Extension() string { return ".gz" }
34 | func (Gz) MediaType() string { return "application/gzip" }
35 | 
36 | func (gz Gz) Match(_ context.Context, filename string, stream io.Reader) (MatchResult, error) {
37 | 	var mr MatchResult
38 | 
39 | 	// match filename
40 | 	if strings.Contains(strings.ToLower(filename), gz.Extension()) {
41 | 		mr.ByName = true
42 | 	}
43 | 
44 | 	// match file header
45 | 	buf, err := readAtMost(stream, len(gzHeader))
46 | 	if err != nil {
47 | 		return mr, err
48 | 	}
49 | 	mr.ByStream = bytes.Equal(buf, gzHeader)
50 | 
51 | 	return mr, nil
52 | }
53 | 
54 | func (gz Gz) OpenWriter(w io.Writer) (io.WriteCloser, error) {
55 | 	// assume default compression level if 0, rather than no
56 | 	// compression, since no compression on a gzipped file
57 | 	// doesn't make any sense in our use cases
58 | 	level := gz.CompressionLevel
59 | 	if level == 0 {
60 | 		level = gzip.DefaultCompression
61 | 	}
62 | 
63 | 	var wc io.WriteCloser
64 | 	var err error
65 | 	if gz.Multithreaded {
66 | 		wc, err = pgzip.NewWriterLevel(w, level)
67 | 	} else {
68 | 		wc, err = gzip.NewWriterLevel(w, level)
69 | 	}
70 | 	return wc, err
71 | }
72 | 
73 | func (gz Gz) OpenReader(r io.Reader) (io.ReadCloser, error) {
74 | 	if gz.Multithreaded {
75 | 		gzR, err := pgzip.NewReader(r)
76 | 		if gzR != nil && gz.DisableMultistream {
77 | 			gzR.Multistream(false)
78 | 		}
79 | 		return gzR, err
80 | 	}
81 | 
82 | 	gzR, err := gzip.NewReader(r)
83 | 	if gzR != nil && gz.DisableMultistream {
84 | 		gzR.Multistream(false)
85 | 	}
86 | 	return gzR, err
87 | }
88 | 
89 | // magic number at the beginning of gzip files
90 | var gzHeader = []byte{0x1f, 0x8b}
```

interfaces.go
```
1 | package archives
2 | 
3 | import (
4 | 	"context"
5 | 	"io"
6 | )
7 | 
8 | // Format represents a way of getting data out of something else.
9 | // A format usually represents compression or an archive (or both).
10 | type Format interface {
11 | 	// Extension returns the conventional file extension for this
12 | 	// format.
13 | 	Extension() string
14 | 
15 | 	// MediaType returns the MIME type ("content type") of this
16 | 	// format (see RFC 2046).
17 | 	MediaType() string
18 | 
19 | 	// Match returns true if the given name/stream is recognized.
20 | 	// One of the arguments is optional: filename might be empty
21 | 	// if working with an unnamed stream, or stream might be empty
22 | 	// if only working with a file on disk; but both may also be
23 | 	// specified. The filename should consist only of the base name,
24 | 	// not path components, and is typically used for matching by
25 | 	// file extension. However, matching by reading the stream is
26 | 	// preferred as it is more accurate. Match reads only as many
27 | 	// bytes as needed to determine a match.
28 | 	Match(ctx context.Context, filename string, stream io.Reader) (MatchResult, error)
29 | }
30 | 
31 | // Compression is a compression format with both compress and decompress methods.
32 | type Compression interface {
33 | 	Format
34 | 	Compressor
35 | 	Decompressor
36 | }
37 | 
38 | // Archival is an archival format that can create/write archives.
39 | type Archival interface {
40 | 	Format
41 | 	Archiver
42 | 	Extractor
43 | }
44 | 
45 | // Extraction is an archival format that extract from (read) archives.
46 | type Extraction interface {
47 | 	Format
48 | 	Extractor
49 | }
50 | 
51 | // Compressor can compress data by wrapping a writer.
52 | type Compressor interface {
53 | 	// OpenWriter wraps w with a new writer that compresses what is written.
54 | 	// The writer must be closed when writing is finished.
55 | 	OpenWriter(w io.Writer) (io.WriteCloser, error)
56 | }
57 | 
58 | // Decompressor can decompress data by wrapping a reader.
59 | type Decompressor interface {
60 | 	// OpenReader wraps r with a new reader that decompresses what is read.
61 | 	// The reader must be closed when reading is finished.
62 | 	OpenReader(r io.Reader) (io.ReadCloser, error)
63 | }
64 | 
65 | // Archiver can create a new archive.
66 | type Archiver interface {
67 | 	// Archive writes an archive file to output with the given files.
68 | 	//
69 | 	// Context cancellation must be honored.
70 | 	Archive(ctx context.Context, output io.Writer, files []FileInfo) error
71 | }
72 | 
73 | // ArchiveAsyncJob contains a File to be archived and a channel that
74 | // the result of the archiving should be returned on.
75 | // EXPERIMENTAL: Subject to change or removal.
76 | type ArchiveAsyncJob struct {
77 | 	File   FileInfo
78 | 	Result chan<- error
79 | }
80 | 
81 | // ArchiverAsync is an Archiver that can also create archives
82 | // asynchronously by pumping files into a channel as they are
83 | // discovered.
84 | // EXPERIMENTAL: Subject to change or removal.
85 | type ArchiverAsync interface {
86 | 	Archiver
87 | 
88 | 	// Use ArchiveAsync if you can't pre-assemble a list of all
89 | 	// the files for the archive. Close the jobs channel after
90 | 	// all the files have been sent.
91 | 	//
92 | 	// This won't return until the channel is closed.
93 | 	ArchiveAsync(ctx context.Context, output io.Writer, jobs <-chan ArchiveAsyncJob) error
94 | }
95 | 
96 | // Extractor can extract files from an archive.
97 | type Extractor interface {
98 | 	// Extract walks entries in the archive and calls handleFile for each
99 | 	// entry in the archive.
100 | 	//
101 | 	// Any files opened in the FileHandler should be closed when it returns,
102 | 	// as there is no guarantee the files can be read outside the handler
103 | 	// or after the walk has proceeded to the next file.
104 | 	//
105 | 	// Context cancellation must be honored.
106 | 	Extract(ctx context.Context, archive io.Reader, handleFile FileHandler) error
107 | }
108 | 
109 | // Inserter can insert files into an existing archive.
110 | // EXPERIMENTAL: Subject to change.
111 | type Inserter interface {
112 | 	// Insert inserts the files into archive.
113 | 	//
114 | 	// Context cancellation must be honored.
115 | 	Insert(ctx context.Context, archive io.ReadWriteSeeker, files []FileInfo) error
116 | }
```

lz4.go
```
1 | package archives
2 | 
3 | import (
4 | 	"bytes"
5 | 	"context"
6 | 	"io"
7 | 	"strings"
8 | 
9 | 	"github.com/pierrec/lz4/v4"
10 | )
11 | 
12 | func init() {
13 | 	RegisterFormat(Lz4{})
14 | }
15 | 
16 | // Lz4 facilitates LZ4 compression.
17 | type Lz4 struct {
18 | 	CompressionLevel int
19 | }
20 | 
21 | func (Lz4) Extension() string { return ".lz4" }
22 | func (Lz4) MediaType() string { return "application/x-lz4" }
23 | 
24 | func (lz Lz4) Match(_ context.Context, filename string, stream io.Reader) (MatchResult, error) {
25 | 	var mr MatchResult
26 | 
27 | 	// match filename
28 | 	if strings.Contains(strings.ToLower(filename), lz.Extension()) {
29 | 		mr.ByName = true
30 | 	}
31 | 
32 | 	// match file header
33 | 	buf, err := readAtMost(stream, len(lz4Header))
34 | 	if err != nil {
35 | 		return mr, err
36 | 	}
37 | 	mr.ByStream = bytes.Equal(buf, lz4Header)
38 | 
39 | 	return mr, nil
40 | }
41 | 
42 | func (lz Lz4) OpenWriter(w io.Writer) (io.WriteCloser, error) {
43 | 	lzw := lz4.NewWriter(w)
44 | 	options := []lz4.Option{
45 | 		lz4.CompressionLevelOption(lz4.CompressionLevel(lz.CompressionLevel)),
46 | 	}
47 | 	if err := lzw.Apply(options...); err != nil {
48 | 		return nil, err
49 | 	}
50 | 	return lzw, nil
51 | }
52 | 
53 | func (Lz4) OpenReader(r io.Reader) (io.ReadCloser, error) {
54 | 	return io.NopCloser(lz4.NewReader(r)), nil
55 | }
56 | 
57 | var lz4Header = []byte{0x04, 0x22, 0x4d, 0x18}
```

lzip.go
```
1 | package archives
2 | 
3 | import (
4 | 	"bytes"
5 | 	"context"
6 | 	"io"
7 | 	"path/filepath"
8 | 	"strings"
9 | 
10 | 	"github.com/sorairolake/lzip-go"
11 | )
12 | 
13 | func init() {
14 | 	RegisterFormat(Lzip{})
15 | }
16 | 
17 | // Lzip facilitates lzip compression.
18 | type Lzip struct{}
19 | 
20 | func (Lzip) Extension() string { return ".lz" }
21 | func (Lzip) MediaType() string { return "application/x-lzip" }
22 | 
23 | func (lz Lzip) Match(_ context.Context, filename string, stream io.Reader) (MatchResult, error) {
24 | 	var mr MatchResult
25 | 
26 | 	// match filename
27 | 	if filepath.Ext(strings.ToLower(filename)) == lz.Extension() {
28 | 		mr.ByName = true
29 | 	}
30 | 
31 | 	// match file header
32 | 	buf, err := readAtMost(stream, len(lzipHeader))
33 | 	if err != nil {
34 | 		return mr, err
35 | 	}
36 | 	mr.ByStream = bytes.Equal(buf, lzipHeader)
37 | 
38 | 	return mr, nil
39 | }
40 | 
41 | func (Lzip) OpenWriter(w io.Writer) (io.WriteCloser, error) {
42 | 	return lzip.NewWriter(w), nil
43 | }
44 | 
45 | func (Lzip) OpenReader(r io.Reader) (io.ReadCloser, error) {
46 | 	lzr, err := lzip.NewReader(r)
47 | 	if err != nil {
48 | 		return nil, err
49 | 	}
50 | 	return io.NopCloser(lzr), err
51 | }
52 | 
53 | // magic number at the beginning of lzip files
54 | // https://datatracker.ietf.org/doc/html/draft-diaz-lzip-09#section-2
55 | var lzipHeader = []byte("LZIP")
```

minlz.go
```
1 | package archives
2 | 
3 | import (
4 | 	"bytes"
5 | 	"context"
6 | 	"io"
7 | 	"path/filepath"
8 | 	"strings"
9 | 
10 | 	"github.com/minio/minlz"
11 | )
12 | 
13 | func init() {
14 | 	RegisterFormat(MinLZ{})
15 | }
16 | 
17 | // MinLZ facilitates MinLZ compression. See
18 | // https://github.com/minio/minlz/blob/main/SPEC.md
19 | // and
20 | // https://blog.min.io/minlz-compression-algorithm/.
21 | type MinLZ struct{}
22 | 
23 | func (MinLZ) Extension() string { return ".mz" }
24 | func (MinLZ) MediaType() string { return "application/x-minlz-compressed" }
25 | 
26 | func (mz MinLZ) Match(_ context.Context, filename string, stream io.Reader) (MatchResult, error) {
27 | 	var mr MatchResult
28 | 
29 | 	// match filename
30 | 	if filepath.Ext(strings.ToLower(filename)) == ".mz" {
31 | 		mr.ByName = true
32 | 	}
33 | 
34 | 	// match file header
35 | 	buf, err := readAtMost(stream, len(mzHeader))
36 | 	if err != nil {
37 | 		return mr, err
38 | 	}
39 | 	mr.ByStream = bytes.Equal(buf, mzHeader)
40 | 
41 | 	return mr, nil
42 | }
43 | 
44 | func (MinLZ) OpenWriter(w io.Writer) (io.WriteCloser, error) {
45 | 	return minlz.NewWriter(w), nil
46 | }
47 | 
48 | func (MinLZ) OpenReader(r io.Reader) (io.ReadCloser, error) {
49 | 	mr := minlz.NewReader(r)
50 | 	return io.NopCloser(mr), nil
51 | }
52 | 
53 | var mzHeader = []byte("\xff\x06\x00\x00MinLz")
```

rar.go
```
1 | package archives
2 | 
3 | import (
4 | 	"bytes"
5 | 	"context"
6 | 	"errors"
7 | 	"fmt"
8 | 	"io"
9 | 	"io/fs"
10 | 	"log"
11 | 	"os"
12 | 	"path"
13 | 	"strings"
14 | 	"time"
15 | 
16 | 	"github.com/nwaples/rardecode/v2"
17 | )
18 | 
19 | func init() {
20 | 	RegisterFormat(Rar{})
21 | }
22 | 
23 | type rarReader interface {
24 | 	Next() (*rardecode.FileHeader, error)
25 | 	io.Reader
26 | 	io.WriterTo
27 | }
28 | 
29 | type Rar struct {
30 | 	// If true, errors encountered during reading or writing
31 | 	// a file within an archive will be logged and the
32 | 	// operation will continue on remaining files.
33 | 	ContinueOnError bool
34 | 
35 | 	// Password to open archives.
36 | 	Password string
37 | 
38 | 	// Name for a multi-volume archive. When Name is specified,
39 | 	// the named file is extracted (rather than any io.Reader that
40 | 	// may be passed to Extract). If the archive is a multi-volume
41 | 	// archive, this name will also be used by the decoder to derive
42 | 	// the filename of the next volume in the volume set.
43 | 	Name string
44 | 
45 | 	// FS is an fs.FS exposing the files of the archive. Unless Name is
46 | 	// also specified, this does nothing. When Name is also specified,
47 | 	// FS defines the fs.FS that from which the archive will be opened,
48 | 	// and in the case of a multi-volume archive, from where each subsequent
49 | 	// volume of the volume set will be loaded.
50 | 	//
51 | 	// Typically this should be a DirFS pointing at the directory containing
52 | 	// the volumes of the archive.
53 | 	FS fs.FS
54 | }
55 | 
56 | func (Rar) Extension() string { return ".rar" }
57 | func (Rar) MediaType() string { return "application/vnd.rar" }
58 | 
59 | func (r Rar) Match(_ context.Context, filename string, stream io.Reader) (MatchResult, error) {
60 | 	var mr MatchResult
61 | 
62 | 	// match filename
63 | 	if strings.Contains(strings.ToLower(filename), r.Extension()) {
64 | 		mr.ByName = true
65 | 	}
66 | 
67 | 	// match file header (there are two versions; allocate buffer for larger one)
68 | 	buf, err := readAtMost(stream, len(rarHeaderV5_0))
69 | 	if err != nil {
70 | 		return mr, err
71 | 	}
72 | 
73 | 	matchedV1_5 := len(buf) >= len(rarHeaderV1_5) &&
74 | 		bytes.Equal(rarHeaderV1_5, buf[:len(rarHeaderV1_5)])
75 | 	matchedV5_0 := len(buf) >= len(rarHeaderV5_0) &&
76 | 		bytes.Equal(rarHeaderV5_0, buf[:len(rarHeaderV5_0)])
77 | 
78 | 	mr.ByStream = matchedV1_5 || matchedV5_0
79 | 
80 | 	return mr, nil
81 | }
82 | 
83 | // Archive is not implemented for RAR because it is patent-encumbered.
84 | 
85 | func (r Rar) Extract(ctx context.Context, sourceArchive io.Reader, handleFile FileHandler) error {
86 | 	var options []rardecode.Option
87 | 	if r.Password != "" {
88 | 		options = append(options, rardecode.Password(r.Password))
89 | 	}
90 | 
91 | 	if r.FS != nil {
92 | 		options = append(options, rardecode.FileSystem(r.FS))
93 | 	}
94 | 
95 | 	var (
96 | 		rr  rarReader
97 | 		err error
98 | 	)
99 | 
100 | 	// If a name has been provided, then the sourceArchive stream is ignored
101 | 	// and the archive is opened directly via the filesystem (or provided FS).
102 | 	if r.Name != "" {
103 | 		var or *rardecode.ReadCloser
104 | 		if or, err = rardecode.OpenReader(r.Name, options...); err == nil {
105 | 			rr = or
106 | 			defer or.Close()
107 | 		}
108 | 	} else {
109 | 		rr, err = rardecode.NewReader(sourceArchive, options...)
110 | 	}
111 | 	if err != nil {
112 | 		return err
113 | 	}
114 | 
115 | 	// important to initialize to non-nil, empty value due to how fileIsIncluded works
116 | 	skipDirs := skipList{}
117 | 
118 | 	for {
119 | 		if err := ctx.Err(); err != nil {
120 | 			return err // honor context cancellation
121 | 		}
122 | 
123 | 		hdr, err := rr.Next()
124 | 		if err == io.EOF {
125 | 			break
126 | 		}
127 | 		if err != nil {
128 | 			if r.ContinueOnError {
129 | 				log.Printf("[ERROR] Advancing to next file in rar archive: %v", err)
130 | 				continue
131 | 			}
132 | 			return err
133 | 		}
134 | 		if fileIsIncluded(skipDirs, hdr.Name) {
135 | 			continue
136 | 		}
137 | 
138 | 		info := rarFileInfo{hdr}
139 | 		file := FileInfo{
140 | 			FileInfo:      info,
141 | 			Header:        hdr,
142 | 			NameInArchive: hdr.Name,
143 | 			Open: func() (fs.File, error) {
144 | 				return fileInArchive{io.NopCloser(rr), info}, nil
145 | 			},
146 | 		}
147 | 
148 | 		err = handleFile(ctx, file)
149 | 		if errors.Is(err, fs.SkipAll) {
150 | 			break
151 | 		} else if errors.Is(err, fs.SkipDir) && file.IsDir() {
152 | 			skipDirs.add(hdr.Name)
153 | 		} else if err != nil {
154 | 			return fmt.Errorf("handling file: %s: %w", hdr.Name, err)
155 | 		}
156 | 	}
157 | 
158 | 	return nil
159 | }
160 | 
161 | // rarFileInfo satisfies the fs.FileInfo interface for RAR entries.
162 | type rarFileInfo struct {
163 | 	fh *rardecode.FileHeader
164 | }
165 | 
166 | func (rfi rarFileInfo) Name() string       { return path.Base(rfi.fh.Name) }
167 | func (rfi rarFileInfo) Size() int64        { return rfi.fh.UnPackedSize }
168 | func (rfi rarFileInfo) Mode() os.FileMode  { return rfi.fh.Mode() }
169 | func (rfi rarFileInfo) ModTime() time.Time { return rfi.fh.ModificationTime }
170 | func (rfi rarFileInfo) IsDir() bool        { return rfi.fh.IsDir }
171 | func (rfi rarFileInfo) Sys() any           { return nil }
172 | 
173 | var (
174 | 	rarHeaderV1_5 = []byte("Rar!\x1a\x07\x00")     // v1.5
175 | 	rarHeaderV5_0 = []byte("Rar!\x1a\x07\x01\x00") // v5.0
176 | )
177 | 
178 | // Interface guard
179 | var _ Extractor = Rar{}
```

rar_test.go
```
1 | package archives
2 | 
3 | import (
4 | 	"context"
5 | 	"crypto/sha1"
6 | 	"encoding/hex"
7 | 	"io"
8 | 	"testing"
9 | )
10 | 
11 | func TestRarExtractMultiVolume(t *testing.T) {
12 | 	// Test files testdata/test.part*.rar were created by:
13 | 	//   seq 0 2000 > test.txt
14 | 	//   rar a -v1k test.rar test.txt
15 | 	rar := Rar{
16 | 		Name: "test.part01.rar",
17 | 		FS:   DirFS("testdata"),
18 | 	}
19 | 
20 | 	const expectedSHA1Sum = "4da7f88f69b44a3fdb705667019a65f4c6e058a3"
21 | 	if err := rar.Extract(context.Background(), nil, func(_ context.Context, info FileInfo) error {
22 | 		f, err := info.Open()
23 | 		if err != nil {
24 | 			return err
25 | 		}
26 | 		defer f.Close()
27 | 
28 | 		h := sha1.New()
29 | 		if _, err = io.Copy(h, f); err != nil {
30 | 			return err
31 | 		}
32 | 
33 | 		if got := hex.EncodeToString(h.Sum(nil)); got != expectedSHA1Sum {
34 | 			t.Errorf("expected %s, got %s", expectedSHA1Sum, got)
35 | 		}
36 | 		return nil
37 | 	}); err != nil {
38 | 		t.Error(err)
39 | 	}
40 | }
```

sz.go
```
1 | package archives
2 | 
3 | import (
4 | 	"bytes"
5 | 	"context"
6 | 	"io"
7 | 	"strings"
8 | 
9 | 	"github.com/klauspost/compress/s2"
10 | )
11 | 
12 | func init() {
13 | 	RegisterFormat(Sz{})
14 | }
15 | 
16 | // Sz facilitates Snappy compression. It uses S2
17 | // for reading and writing, but by default will
18 | // write Snappy-compatible data.
19 | type Sz struct {
20 | 	// Configurable S2 extension.
21 | 	S2 S2
22 | }
23 | 
24 | // S2 is an extension of Snappy that can read Snappy
25 | // streams and write Snappy-compatible streams, but
26 | // can also be configured to write Snappy-incompatible
27 | // streams for greater gains. See
28 | // https://pkg.go.dev/github.com/klauspost/compress/s2
29 | // for details and the documentation for each option.
30 | type S2 struct {
31 | 	// reader options
32 | 	MaxBlockSize           int
33 | 	AllocBlock             int
34 | 	IgnoreStreamIdentifier bool
35 | 	IgnoreCRC              bool
36 | 
37 | 	// writer options
38 | 	AddIndex           bool
39 | 	Compression        S2Level
40 | 	BlockSize          int
41 | 	Concurrency        int
42 | 	FlushOnWrite       bool
43 | 	Padding            int
44 | 	SnappyIncompatible bool
45 | }
46 | 
47 | func (Sz) Extension() string { return ".sz" }
48 | func (Sz) MediaType() string { return "application/x-snappy-framed" }
49 | 
50 | func (sz Sz) Match(_ context.Context, filename string, stream io.Reader) (MatchResult, error) {
51 | 	var mr MatchResult
52 | 
53 | 	// match filename
54 | 	if strings.Contains(strings.ToLower(filename), sz.Extension()) ||
55 | 		strings.Contains(strings.ToLower(filename), ".s2") {
56 | 		mr.ByName = true
57 | 	}
58 | 
59 | 	// match file header
60 | 	buf, err := readAtMost(stream, len(snappyHeader))
61 | 	if err != nil {
62 | 		return mr, err
63 | 	}
64 | 	mr.ByStream = bytes.Equal(buf, snappyHeader)
65 | 
66 | 	return mr, nil
67 | }
68 | 
69 | func (sz Sz) OpenWriter(w io.Writer) (io.WriteCloser, error) {
70 | 	var opts []s2.WriterOption
71 | 	if sz.S2.AddIndex {
72 | 		opts = append(opts, s2.WriterAddIndex())
73 | 	}
74 | 	switch sz.S2.Compression {
75 | 	case S2LevelNone:
76 | 		opts = append(opts, s2.WriterUncompressed())
77 | 	case S2LevelBetter:
78 | 		opts = append(opts, s2.WriterBetterCompression())
79 | 	case S2LevelBest:
80 | 		opts = append(opts, s2.WriterBestCompression())
81 | 	}
82 | 	if sz.S2.BlockSize != 0 {
83 | 		opts = append(opts, s2.WriterBlockSize(sz.S2.BlockSize))
84 | 	}
85 | 	if sz.S2.Concurrency != 0 {
86 | 		opts = append(opts, s2.WriterConcurrency(sz.S2.Concurrency))
87 | 	}
88 | 	if sz.S2.FlushOnWrite {
89 | 		opts = append(opts, s2.WriterFlushOnWrite())
90 | 	}
91 | 	if sz.S2.Padding != 0 {
92 | 		opts = append(opts, s2.WriterPadding(sz.S2.Padding))
93 | 	}
94 | 	if !sz.S2.SnappyIncompatible {
95 | 		// this option is inverted because by default we should
96 | 		// probably write Snappy-compatible streams
97 | 		opts = append(opts, s2.WriterSnappyCompat())
98 | 	}
99 | 	return s2.NewWriter(w, opts...), nil
100 | }
101 | 
102 | func (sz Sz) OpenReader(r io.Reader) (io.ReadCloser, error) {
103 | 	var opts []s2.ReaderOption
104 | 	if sz.S2.AllocBlock != 0 {
105 | 		opts = append(opts, s2.ReaderAllocBlock(sz.S2.AllocBlock))
106 | 	}
107 | 	if sz.S2.IgnoreCRC {
108 | 		opts = append(opts, s2.ReaderIgnoreCRC())
109 | 	}
110 | 	if sz.S2.IgnoreStreamIdentifier {
111 | 		opts = append(opts, s2.ReaderIgnoreStreamIdentifier())
112 | 	}
113 | 	if sz.S2.MaxBlockSize != 0 {
114 | 		opts = append(opts, s2.ReaderMaxBlockSize(sz.S2.MaxBlockSize))
115 | 	}
116 | 	return io.NopCloser(s2.NewReader(r, opts...)), nil
117 | }
118 | 
119 | // Compression level for S2 (Snappy/Sz extension).
120 | // EXPERIMENTAL: May be changed or removed without a major version bump.
121 | type S2Level int
122 | 
123 | // Compression levels for S2.
124 | // EXPERIMENTAL: May be changed or removed without a major version bump.
125 | const (
126 | 	S2LevelNone   S2Level = 0
127 | 	S2LevelFast   S2Level = 1
128 | 	S2LevelBetter S2Level = 2
129 | 	S2LevelBest   S2Level = 3
130 | )
131 | 
132 | // https://github.com/google/snappy/blob/master/framing_format.txt - contains "sNaPpY"
133 | var snappyHeader = []byte{0xff, 0x06, 0x00, 0x00, 0x73, 0x4e, 0x61, 0x50, 0x70, 0x59}
```

tar.go
```
1 | package archives
2 | 
3 | import (
4 | 	"archive/tar"
5 | 	"context"
6 | 	"errors"
7 | 	"fmt"
8 | 	"io"
9 | 	"io/fs"
10 | 	"log"
11 | 	"strings"
12 | )
13 | 
14 | func init() {
15 | 	RegisterFormat(Tar{})
16 | }
17 | 
18 | type Tar struct {
19 | 	// If true, use GNU header format
20 | 	FormatGNU bool
21 | 
22 | 	// If true, preserve only numeric user and group id
23 | 	NumericUIDGID bool
24 | 
25 | 	// If true, errors encountered during reading or writing
26 | 	// a file within an archive will be logged and the
27 | 	// operation will continue on remaining files.
28 | 	ContinueOnError bool
29 | 
30 | 	// User ID of the file owner
31 | 	Uid int
32 | 
33 | 	// Group ID of the file owner
34 | 	Gid int
35 | 
36 | 	// Username of the file owner
37 | 	Uname string
38 | 
39 | 	// Group name of the file owner
40 | 	Gname string
41 | }
42 | 
43 | func (Tar) Extension() string { return ".tar" }
44 | func (Tar) MediaType() string { return "application/x-tar" }
45 | 
46 | func (t Tar) Match(_ context.Context, filename string, stream io.Reader) (MatchResult, error) {
47 | 	var mr MatchResult
48 | 
49 | 	// match filename
50 | 	if strings.Contains(strings.ToLower(filename), t.Extension()) {
51 | 		mr.ByName = true
52 | 	}
53 | 
54 | 	// match file header
55 | 	if stream != nil {
56 | 		r := tar.NewReader(stream)
57 | 		_, err := r.Next()
58 | 		mr.ByStream = err == nil
59 | 	}
60 | 
61 | 	return mr, nil
62 | }
63 | 
64 | func (t Tar) Archive(ctx context.Context, output io.Writer, files []FileInfo) error {
65 | 	tw := tar.NewWriter(output)
66 | 	defer tw.Close()
67 | 
68 | 	for _, file := range files {
69 | 		if err := t.writeFileToArchive(ctx, tw, file); err != nil {
70 | 			if t.ContinueOnError && ctx.Err() == nil { // context errors should always abort
71 | 				log.Printf("[ERROR] %v", err)
72 | 				continue
73 | 			}
74 | 			return err
75 | 		}
76 | 	}
77 | 
78 | 	return nil
79 | }
80 | 
81 | func (t Tar) ArchiveAsync(ctx context.Context, output io.Writer, jobs <-chan ArchiveAsyncJob) error {
82 | 	tw := tar.NewWriter(output)
83 | 	defer tw.Close()
84 | 
85 | 	for job := range jobs {
86 | 		job.Result <- t.writeFileToArchive(ctx, tw, job.File)
87 | 	}
88 | 
89 | 	return nil
90 | }
91 | 
92 | func (t Tar) writeFileToArchive(ctx context.Context, tw *tar.Writer, file FileInfo) error {
93 | 	if err := ctx.Err(); err != nil {
94 | 		return err // honor context cancellation
95 | 	}
96 | 
97 | 	hdr, err := tar.FileInfoHeader(file, file.LinkTarget)
98 | 	if err != nil {
99 | 		return fmt.Errorf("file %s: creating header: %w", file.NameInArchive, err)
100 | 	}
101 | 	hdr.Name = file.NameInArchive // complete path, since FileInfoHeader() only has base name
102 | 	if hdr.Name == "" {
103 | 		hdr.Name = file.Name() // assume base name of file I guess
104 | 	}
105 | 	if t.FormatGNU {
106 | 		hdr.Format = tar.FormatGNU
107 | 	}
108 | 	if t.NumericUIDGID {
109 | 		hdr.Uname = ""
110 | 		hdr.Gname = ""
111 | 	}
112 | 	if t.Uid != 0 {
113 | 		hdr.Uid = t.Uid
114 | 	}
115 | 	if t.Gid != 0 {
116 | 		hdr.Gid = t.Gid
117 | 	}
118 | 	if t.Uname != "" {
119 | 		hdr.Uname = t.Uname
120 | 	}
121 | 	if t.Gname != "" {
122 | 		hdr.Gname = t.Gname
123 | 	}
124 | 
125 | 	if err := tw.WriteHeader(hdr); err != nil {
126 | 		return fmt.Errorf("file %s: writing header: %w", file.NameInArchive, err)
127 | 	}
128 | 
129 | 	// only proceed to write a file body if there is actually a body
130 | 	// (for example, directories and links don't have a body)
131 | 	if hdr.Typeflag != tar.TypeReg {
132 | 		return nil
133 | 	}
134 | 
135 | 	if err := openAndCopyFile(file, tw); err != nil {
136 | 		return fmt.Errorf("file %s: writing data: %w", file.NameInArchive, err)
137 | 	}
138 | 
139 | 	return nil
140 | }
141 | 
142 | func (t Tar) Insert(ctx context.Context, into io.ReadWriteSeeker, files []FileInfo) error {
143 | 	// Tar files may end with some, none, or a lot of zero-byte padding. The spec says
144 | 	// it should end with two 512-byte trailer records consisting solely of null/0
145 | 	// bytes: https://www.gnu.org/software/tar/manual/html_node/Standard.html. However,
146 | 	// in my experiments using the `tar` command, I've found that is not the case,
147 | 	// and Colin Percival (author of tarsnap) confirmed this:
148 | 	// - https://twitter.com/cperciva/status/1476774314623913987
149 | 	// - https://twitter.com/cperciva/status/1476776999758663680
150 | 	// So while this solution on Stack Overflow makes sense if you control the
151 | 	// writer: https://stackoverflow.com/a/18330903/1048862 - and I did get it
152 | 	// to work in that case -- it is not a general solution. Seems that the only
153 | 	// reliable thing to do is scan the entire archive to find the last file,
154 | 	// read its size, then use that to compute the end of content and thus the
155 | 	// true length of end-of-archive padding. This is slightly more complex than
156 | 	// just adding the size of the last file to the current stream/seek position,
157 | 	// because we have to align to 512-byte blocks precisely. I don't actually
158 | 	// fully know why this works, but in my testing on a few different files it
159 | 	// did work, whereas other solutions only worked on 1 specific file. *shrug*
160 | 	//
161 | 	// Another option is to scan the file for the last contiguous series of 0s,
162 | 	// without interpreting the tar format at all, and to find the nearest
163 | 	// blocksize-offset and start writing there. Problem is that you wouldn't
164 | 	// know if you just overwrote some of the last file if it ends with all 0s.
165 | 	// Sigh.
166 | 	var lastFileSize, lastStreamPos int64
167 | 	tr := tar.NewReader(into)
168 | 	for {
169 | 		hdr, err := tr.Next()
170 | 		if err == io.EOF {
171 | 			break
172 | 		}
173 | 		if err != nil {
174 | 			return err
175 | 		}
176 | 		lastStreamPos, err = into.Seek(0, io.SeekCurrent)
177 | 		if err != nil {
178 | 			return err
179 | 		}
180 | 		lastFileSize = hdr.Size
181 | 	}
182 | 
183 | 	// we can now compute the precise location to write the new file to (I think)
184 | 	const blockSize = 512 // (as of Go 1.17, this is also a hard-coded const in the archive/tar package)
185 | 	newOffset := lastStreamPos + lastFileSize
186 | 	newOffset += blockSize - (newOffset % blockSize) // shift to next-nearest block boundary
187 | 	_, err := into.Seek(newOffset, io.SeekStart)
188 | 	if err != nil {
189 | 		return err
190 | 	}
191 | 
192 | 	tw := tar.NewWriter(into)
193 | 	defer tw.Close()
194 | 
195 | 	for i, file := range files {
196 | 		if err := ctx.Err(); err != nil {
197 | 			return err // honor context cancellation
198 | 		}
199 | 		err = t.writeFileToArchive(ctx, tw, file)
200 | 		if err != nil {
201 | 			if t.ContinueOnError && ctx.Err() == nil {
202 | 				log.Printf("[ERROR] appending file %d into archive: %s: %v", i, file.Name(), err)
203 | 				continue
204 | 			}
205 | 			return fmt.Errorf("appending file %d into archive: %s: %w", i, file.Name(), err)
206 | 		}
207 | 	}
208 | 
209 | 	return nil
210 | }
211 | 
212 | func (t Tar) Extract(ctx context.Context, sourceArchive io.Reader, handleFile FileHandler) error {
213 | 	tr := tar.NewReader(sourceArchive)
214 | 
215 | 	// important to initialize to non-nil, empty value due to how fileIsIncluded works
216 | 	skipDirs := skipList{}
217 | 
218 | 	for {
219 | 		if err := ctx.Err(); err != nil {
220 | 			return err // honor context cancellation
221 | 		}
222 | 
223 | 		hdr, err := tr.Next()
224 | 		if err == io.EOF {
225 | 			break
226 | 		}
227 | 		if err != nil {
228 | 			if t.ContinueOnError && ctx.Err() == nil {
229 | 				log.Printf("[ERROR] Advancing to next file in tar archive: %v", err)
230 | 				continue
231 | 			}
232 | 			return err
233 | 		}
234 | 		if fileIsIncluded(skipDirs, hdr.Name) {
235 | 			continue
236 | 		}
237 | 		if hdr.Typeflag == tar.TypeXGlobalHeader {
238 | 			// ignore the pax global header from git-generated tarballs
239 | 			continue
240 | 		}
241 | 
242 | 		info := hdr.FileInfo()
243 | 		file := FileInfo{
244 | 			FileInfo:      info,
245 | 			Header:        hdr,
246 | 			NameInArchive: hdr.Name,
247 | 			LinkTarget:    hdr.Linkname,
248 | 			Open: func() (fs.File, error) {
249 | 				return fileInArchive{io.NopCloser(tr), info}, nil
250 | 			},
251 | 		}
252 | 
253 | 		err = handleFile(ctx, file)
254 | 		if errors.Is(err, fs.SkipAll) {
255 | 			// At first, I wasn't sure if fs.SkipAll implied that the rest of the entries
256 | 			// should still be iterated and just "skipped" (i.e. no-ops) or if the walk
257 | 			// should stop; both have the same net effect, one is just less efficient...
258 | 			// apparently the name of fs.StopWalk was the preferred name, but it still
259 | 			// became fs.SkipAll because of semantics with documentation; see
260 | 			// https://github.com/golang/go/issues/47209 -- anyway, the walk should stop.
261 | 			break
262 | 		} else if errors.Is(err, fs.SkipDir) && file.IsDir() {
263 | 			skipDirs.add(hdr.Name)
264 | 		} else if err != nil {
265 | 			return fmt.Errorf("handling file: %s: %w", hdr.Name, err)
266 | 		}
267 | 	}
268 | 
269 | 	return nil
270 | }
271 | 
272 | // Interface guards
273 | var (
274 | 	_ Archiver      = (*Tar)(nil)
275 | 	_ ArchiverAsync = (*Tar)(nil)
276 | 	_ Extractor     = (*Tar)(nil)
277 | 	_ Inserter      = (*Tar)(nil)
278 | )
```

xz.go
```
1 | package archives
2 | 
3 | import (
4 | 	"bytes"
5 | 	"context"
6 | 	"io"
7 | 	"strings"
8 | 
9 | 	fastxz "github.com/therootcompany/xz"
10 | 	"github.com/ulikunitz/xz"
11 | )
12 | 
13 | func init() {
14 | 	RegisterFormat(Xz{})
15 | }
16 | 
17 | // Xz facilitates xz compression.
18 | type Xz struct{}
19 | 
20 | func (Xz) Extension() string { return ".xz" }
21 | func (Xz) MediaType() string { return "application/x-xz" }
22 | 
23 | func (x Xz) Match(_ context.Context, filename string, stream io.Reader) (MatchResult, error) {
24 | 	var mr MatchResult
25 | 
26 | 	// match filename
27 | 	if strings.Contains(strings.ToLower(filename), x.Extension()) {
28 | 		mr.ByName = true
29 | 	}
30 | 
31 | 	// match file header
32 | 	buf, err := readAtMost(stream, len(xzHeader))
33 | 	if err != nil {
34 | 		return mr, err
35 | 	}
36 | 	mr.ByStream = bytes.Equal(buf, xzHeader)
37 | 
38 | 	return mr, nil
39 | }
40 | 
41 | func (Xz) OpenWriter(w io.Writer) (io.WriteCloser, error) {
42 | 	return xz.NewWriter(w)
43 | }
44 | 
45 | func (Xz) OpenReader(r io.Reader) (io.ReadCloser, error) {
46 | 	xr, err := fastxz.NewReader(r, 0)
47 | 	if err != nil {
48 | 		return nil, err
49 | 	}
50 | 	return io.NopCloser(xr), err
51 | }
52 | 
53 | // magic number at the beginning of xz files; see section 2.1.1.1
54 | // of https://tukaani.org/xz/xz-file-format.txt
55 | var xzHeader = []byte{0xfd, 0x37, 0x7a, 0x58, 0x5a, 0x00}
```

zip.go
```
1 | package archives
2 | 
3 | import (
4 | 	"bytes"
5 | 	"context"
6 | 	"errors"
7 | 	"fmt"
8 | 	"io"
9 | 	"io/fs"
10 | 	"log"
11 | 	"path"
12 | 	"strings"
13 | 
14 | 	szip "github.com/STARRY-S/zip"
15 | 	"golang.org/x/text/encoding"
16 | 
17 | 	"github.com/dsnet/compress/bzip2"
18 | 	"github.com/klauspost/compress/zip"
19 | 	"github.com/klauspost/compress/zstd"
20 | 	"github.com/ulikunitz/xz"
21 | )
22 | 
23 | func init() {
24 | 	RegisterFormat(Zip{})
25 | 
26 | 	// TODO: What about custom flate levels too
27 | 	zip.RegisterCompressor(ZipMethodBzip2, func(out io.Writer) (io.WriteCloser, error) {
28 | 		return bzip2.NewWriter(out, &bzip2.WriterConfig{ /*TODO: Level: z.CompressionLevel*/ })
29 | 	})
30 | 	zip.RegisterCompressor(ZipMethodZstd, func(out io.Writer) (io.WriteCloser, error) {
31 | 		return zstd.NewWriter(out)
32 | 	})
33 | 	zip.RegisterCompressor(ZipMethodXz, func(out io.Writer) (io.WriteCloser, error) {
34 | 		return xz.NewWriter(out)
35 | 	})
36 | 
37 | 	zip.RegisterDecompressor(ZipMethodBzip2, func(r io.Reader) io.ReadCloser {
38 | 		bz2r, err := bzip2.NewReader(r, nil)
39 | 		if err != nil {
40 | 			return nil
41 | 		}
42 | 		return bz2r
43 | 	})
44 | 	zip.RegisterDecompressor(ZipMethodZstd, func(r io.Reader) io.ReadCloser {
45 | 		zr, err := zstd.NewReader(r)
46 | 		if err != nil {
47 | 			return nil
48 | 		}
49 | 		return zr.IOReadCloser()
50 | 	})
51 | 	zip.RegisterDecompressor(ZipMethodXz, func(r io.Reader) io.ReadCloser {
52 | 		xr, err := xz.NewReader(r)
53 | 		if err != nil {
54 | 			return nil
55 | 		}
56 | 		return io.NopCloser(xr)
57 | 	})
58 | }
59 | 
60 | type Zip struct {
61 | 	// Only compress files which are not already in a
62 | 	// compressed format (determined simply by examining
63 | 	// file extension).
64 | 	SelectiveCompression bool
65 | 
66 | 	// The method or algorithm for compressing stored files.
67 | 	Compression uint16
68 | 
69 | 	// If true, errors encountered during reading or writing
70 | 	// a file within an archive will be logged and the
71 | 	// operation will continue on remaining files.
72 | 	ContinueOnError bool
73 | 
74 | 	// For files in zip archives that do not have UTF-8
75 | 	// encoded filenames and comments, specify the character
76 | 	// encoding here.
77 | 	TextEncoding encoding.Encoding
78 | }
79 | 
80 | func (Zip) Extension() string { return ".zip" }
81 | func (Zip) MediaType() string { return "application/zip" }
82 | 
83 | func (z Zip) Match(_ context.Context, filename string, stream io.Reader) (MatchResult, error) {
84 | 	var mr MatchResult
85 | 
86 | 	// match filename
87 | 	if strings.Contains(strings.ToLower(filename), z.Extension()) {
88 | 		mr.ByName = true
89 | 	}
90 | 
91 | 	// match file header
92 | 	for _, hdr := range zipHeaders {
93 | 		buf, err := readAtMost(stream, len(hdr))
94 | 		if err != nil {
95 | 			return mr, err
96 | 		}
97 | 		if bytes.Equal(buf, hdr) {
98 | 			mr.ByStream = true
99 | 			break
100 | 		}
101 | 	}
102 | 
103 | 	return mr, nil
104 | }
105 | 
106 | func (z Zip) Archive(ctx context.Context, output io.Writer, files []FileInfo) error {
107 | 	zw := zip.NewWriter(output)
108 | 	defer zw.Close()
109 | 
110 | 	for i, file := range files {
111 | 		if err := z.archiveOneFile(ctx, zw, i, file); err != nil {
112 | 			return err
113 | 		}
114 | 	}
115 | 
116 | 	return nil
117 | }
118 | 
119 | func (z Zip) ArchiveAsync(ctx context.Context, output io.Writer, jobs <-chan ArchiveAsyncJob) error {
120 | 	zw := zip.NewWriter(output)
121 | 	defer zw.Close()
122 | 
123 | 	var i int
124 | 	for job := range jobs {
125 | 		job.Result <- z.archiveOneFile(ctx, zw, i, job.File)
126 | 		i++
127 | 	}
128 | 
129 | 	return nil
130 | }
131 | 
132 | func (z Zip) archiveOneFile(ctx context.Context, zw *zip.Writer, idx int, file FileInfo) error {
133 | 	if err := ctx.Err(); err != nil {
134 | 		return err // honor context cancellation
135 | 	}
136 | 
137 | 	hdr, err := zip.FileInfoHeader(file)
138 | 	if err != nil {
139 | 		return fmt.Errorf("getting info for file %d: %s: %w", idx, file.Name(), err)
140 | 	}
141 | 	hdr.Name = file.NameInArchive // complete path, since FileInfoHeader() only has base name
142 | 	if hdr.Name == "" {
143 | 		hdr.Name = file.Name() // assume base name of file I guess
144 | 	}
145 | 
146 | 	// customize header based on file properties
147 | 	if file.IsDir() {
148 | 		if !strings.HasSuffix(hdr.Name, "/") {
149 | 			hdr.Name += "/" // required
150 | 		}
151 | 		hdr.Method = zip.Store
152 | 	} else if z.SelectiveCompression {
153 | 		// only enable compression on compressable files
154 | 		ext := strings.ToLower(path.Ext(hdr.Name))
155 | 		if _, ok := compressedFormats[ext]; ok {
156 | 			hdr.Method = zip.Store
157 | 		} else {
158 | 			hdr.Method = z.Compression
159 | 		}
160 | 	} else {
161 | 		hdr.Method = z.Compression
162 | 	}
163 | 
164 | 	w, err := zw.CreateHeader(hdr)
165 | 	if err != nil {
166 | 		return fmt.Errorf("creating header for file %d: %s: %w", idx, file.Name(), err)
167 | 	}
168 | 
169 | 	// directories have no file body
170 | 	if file.IsDir() {
171 | 		return nil
172 | 	}
173 | 	if err := openAndCopyFile(file, w); err != nil {
174 | 		return fmt.Errorf("writing file %d: %s: %w", idx, file.Name(), err)
175 | 	}
176 | 
177 | 	return nil
178 | }
179 | 
180 | // Extract extracts files from z, implementing the Extractor interface. Uniquely, however,
181 | // sourceArchive must be an io.ReaderAt and io.Seeker, which are oddly disjoint interfaces
182 | // from io.Reader which is what the method signature requires. We chose this signature for
183 | // the interface because we figure you can Read() from anything you can ReadAt() or Seek()
184 | // with. Due to the nature of the zip archive format, if sourceArchive is not an io.Seeker
185 | // and io.ReaderAt, an error is returned.
186 | func (z Zip) Extract(ctx context.Context, sourceArchive io.Reader, handleFile FileHandler) error {
187 | 	sra, ok := sourceArchive.(seekReaderAt)
188 | 	if !ok {
189 | 		return fmt.Errorf("input type must be an io.ReaderAt and io.Seeker because of zip format constraints")
190 | 	}
191 | 
192 | 	size, err := streamSizeBySeeking(sra)
193 | 	if err != nil {
194 | 		return fmt.Errorf("determining stream size: %w", err)
195 | 	}
196 | 
197 | 	zr, err := zip.NewReader(sra, size)
198 | 	if err != nil {
199 | 		return err
200 | 	}
201 | 
202 | 	// important to initialize to non-nil, empty value due to how fileIsIncluded works
203 | 	skipDirs := skipList{}
204 | 
205 | 	for i, f := range zr.File {
206 | 		if err := ctx.Err(); err != nil {
207 | 			return err // honor context cancellation
208 | 		}
209 | 
210 | 		// ensure filename and comment are UTF-8 encoded (issue #147 and PR #305)
211 | 		z.decodeText(&f.FileHeader)
212 | 
213 | 		if fileIsIncluded(skipDirs, f.Name) {
214 | 			continue
215 | 		}
216 | 
217 | 		info := f.FileInfo()
218 | 		file := FileInfo{
219 | 			FileInfo:      info,
220 | 			Header:        f.FileHeader,
221 | 			NameInArchive: f.Name,
222 | 			Open: func() (fs.File, error) {
223 | 				openedFile, err := f.Open()
224 | 				if err != nil {
225 | 					return nil, err
226 | 				}
227 | 				return fileInArchive{openedFile, info}, nil
228 | 			},
229 | 		}
230 | 
231 | 		err := handleFile(ctx, file)
232 | 		if errors.Is(err, fs.SkipAll) {
233 | 			break
234 | 		} else if errors.Is(err, fs.SkipDir) && file.IsDir() {
235 | 			skipDirs.add(f.Name)
236 | 		} else if err != nil {
237 | 			if z.ContinueOnError {
238 | 				log.Printf("[ERROR] %s: %v", f.Name, err)
239 | 				continue
240 | 			}
241 | 			return fmt.Errorf("handling file %d: %s: %w", i, f.Name, err)
242 | 		}
243 | 	}
244 | 
245 | 	return nil
246 | }
247 | 
248 | // decodeText decodes the name and comment fields from hdr into UTF-8.
249 | // It is a no-op if the text is already UTF-8 encoded or if z.TextEncoding
250 | // is not specified.
251 | func (z Zip) decodeText(hdr *zip.FileHeader) {
252 | 	if hdr.NonUTF8 && z.TextEncoding != nil {
253 | 		dec := z.TextEncoding.NewDecoder()
254 | 		filename, err := dec.String(hdr.Name)
255 | 		if err == nil {
256 | 			hdr.Name = filename
257 | 		}
258 | 		if hdr.Comment != "" {
259 | 			comment, err := dec.String(hdr.Comment)
260 | 			if err == nil {
261 | 				hdr.Comment = comment
262 | 			}
263 | 		}
264 | 	}
265 | }
266 | 
267 | // Insert appends the listed files into the provided Zip archive stream.
268 | // If the filename already exists in the archive, it will be replaced.
269 | func (z Zip) Insert(ctx context.Context, into io.ReadWriteSeeker, files []FileInfo) error {
270 | 	// following very simple example at https://github.com/STARRY-S/zip?tab=readme-ov-file#usage
271 | 	zu, err := szip.NewUpdater(into)
272 | 	if err != nil {
273 | 		return err
274 | 	}
275 | 	defer zu.Close()
276 | 
277 | 	for idx, file := range files {
278 | 		if err := ctx.Err(); err != nil {
279 | 			return err // honor context cancellation
280 | 		}
281 | 
282 | 		hdr, err := szip.FileInfoHeader(file)
283 | 		if err != nil {
284 | 			return fmt.Errorf("getting info for file %d: %s: %w", idx, file.NameInArchive, err)
285 | 		}
286 | 		hdr.Name = file.NameInArchive // complete path, since FileInfoHeader() only has base name
287 | 		if hdr.Name == "" {
288 | 			hdr.Name = file.Name() // assume base name of file I guess
289 | 		}
290 | 
291 | 		// customize header based on file properties
292 | 		if file.IsDir() {
293 | 			if !strings.HasSuffix(hdr.Name, "/") {
294 | 				hdr.Name += "/" // required
295 | 			}
296 | 			hdr.Method = zip.Store
297 | 		} else if z.SelectiveCompression {
298 | 			// only enable compression on compressable files
299 | 			ext := strings.ToLower(path.Ext(hdr.Name))
300 | 			if _, ok := compressedFormats[ext]; ok {
301 | 				hdr.Method = zip.Store
302 | 			} else {
303 | 				hdr.Method = z.Compression
304 | 			}
305 | 		}
306 | 
307 | 		w, err := zu.Append(hdr.Name, szip.APPEND_MODE_OVERWRITE)
308 | 		if err != nil {
309 | 			return fmt.Errorf("inserting file header: %d: %s: %w", idx, file.Name(), err)
310 | 		}
311 | 
312 | 		// directories have no file body
313 | 		if file.IsDir() {
314 | 			return nil
315 | 		}
316 | 		if err := openAndCopyFile(file, w); err != nil {
317 | 			if z.ContinueOnError && ctx.Err() == nil {
318 | 				log.Printf("[ERROR] appending file %d into archive: %s: %v", idx, file.Name(), err)
319 | 				continue
320 | 			}
321 | 			return fmt.Errorf("copying inserted file %d: %s: %w", idx, file.Name(), err)
322 | 		}
323 | 	}
324 | 
325 | 	return nil
326 | }
327 | 
328 | type seekReaderAt interface {
329 | 	io.ReaderAt
330 | 	io.Seeker
331 | }
332 | 
333 | // Additional compression methods not offered by archive/zip.
334 | // See https://pkware.cachefly.net/webdocs/casestudies/APPNOTE.TXT section 4.4.5.
335 | const (
336 | 	ZipMethodBzip2 = 12
337 | 	// TODO: LZMA: Disabled - because 7z isn't able to unpack ZIP+LZMA ZIP+LZMA2 archives made this way - and vice versa.
338 | 	// ZipMethodLzma     = 14
339 | 	ZipMethodZstd = 93
340 | 	ZipMethodXz   = 95
341 | )
342 | 
343 | // compressedFormats is a (non-exhaustive) set of lowercased
344 | // file extensions for formats that are typically already
345 | // compressed. Compressing files that are already compressed
346 | // is inefficient, so use this set of extensions to avoid that.
347 | var compressedFormats = map[string]struct{}{
348 | 	".7z":   {},
349 | 	".avi":  {},
350 | 	".br":   {},
351 | 	".bz2":  {},
352 | 	".cab":  {},
353 | 	".docx": {},
354 | 	".gif":  {},
355 | 	".gz":   {},
356 | 	".jar":  {},
357 | 	".jpeg": {},
358 | 	".jpg":  {},
359 | 	".lz":   {},
360 | 	".lz4":  {},
361 | 	".lzma": {},
362 | 	".m4v":  {},
363 | 	".mov":  {},
364 | 	".mp3":  {},
365 | 	".mp4":  {},
366 | 	".mpeg": {},
367 | 	".mpg":  {},
368 | 	".png":  {},
369 | 	".pptx": {},
370 | 	".rar":  {},
371 | 	".sz":   {},
372 | 	".tbz2": {},
373 | 	".tgz":  {},
374 | 	".tsz":  {},
375 | 	".txz":  {},
376 | 	".xlsx": {},
377 | 	".xz":   {},
378 | 	".zip":  {},
379 | 	".zipx": {},
380 | }
381 | 
382 | var zipHeaders = [][]byte{
383 | 	[]byte("PK\x03\x04"), // normal
384 | 	[]byte("PK\x05\x06"), // empty
385 | }
386 | 
387 | // Interface guards
388 | var (
389 | 	_ Archiver      = Zip{}
390 | 	_ ArchiverAsync = Zip{}
391 | 	_ Extractor     = Zip{}
392 | )
```

zlib.go
```
1 | package archives
2 | 
3 | import (
4 | 	"context"
5 | 	"io"
6 | 	"strings"
7 | 
8 | 	"github.com/klauspost/compress/zlib"
9 | )
10 | 
11 | func init() {
12 | 	RegisterFormat(Zlib{})
13 | }
14 | 
15 | // Zlib facilitates zlib compression.
16 | type Zlib struct {
17 | 	CompressionLevel int
18 | }
19 | 
20 | func (Zlib) Extension() string { return ".zz" }
21 | func (Zlib) MediaType() string { return "application/zlib" }
22 | 
23 | func (zz Zlib) Match(_ context.Context, filename string, stream io.Reader) (MatchResult, error) {
24 | 	var mr MatchResult
25 | 
26 | 	// match filename
27 | 	if strings.Contains(strings.ToLower(filename), zz.Extension()) {
28 | 		mr.ByName = true
29 | 	}
30 | 
31 | 	// match file header
32 | 	buf, err := readAtMost(stream, 2)
33 | 	// If an error occurred or buf is not 2 bytes we can't check the header
34 | 	if err != nil || len(buf) < 2 {
35 | 		return mr, err
36 | 	}
37 | 
38 | 	mr.ByStream = isValidZlibHeader(buf[0], buf[1])
39 | 
40 | 	return mr, nil
41 | }
42 | 
43 | func (zz Zlib) OpenWriter(w io.Writer) (io.WriteCloser, error) {
44 | 	level := zz.CompressionLevel
45 | 	if level == 0 {
46 | 		level = zlib.DefaultCompression
47 | 	}
48 | 	return zlib.NewWriterLevel(w, level)
49 | }
50 | 
51 | func (Zlib) OpenReader(r io.Reader) (io.ReadCloser, error) {
52 | 	return zlib.NewReader(r)
53 | }
54 | 
55 | func isValidZlibHeader(first, second byte) bool {
56 | 	// Define all 32 valid zlib headers, see https://stackoverflow.com/questions/9050260/what-does-a-zlib-header-look-like/54915442#54915442
57 | 	validHeaders := map[uint16]struct{}{
58 | 		0x081D: {}, 0x085B: {}, 0x0899: {}, 0x08D7: {},
59 | 		0x1819: {}, 0x1857: {}, 0x1895: {}, 0x18D3: {},
60 | 		0x2815: {}, 0x2853: {}, 0x2891: {}, 0x28CF: {},
61 | 		0x3811: {}, 0x384F: {}, 0x388D: {}, 0x38CB: {},
62 | 		0x480D: {}, 0x484B: {}, 0x4889: {}, 0x48C7: {},
63 | 		0x5809: {}, 0x5847: {}, 0x5885: {}, 0x58C3: {},
64 | 		0x6805: {}, 0x6843: {}, 0x6881: {}, 0x68DE: {},
65 | 		0x7801: {}, 0x785E: {}, 0x789C: {}, 0x78DA: {},
66 | 	}
67 | 
68 | 	// Combine the first and second bytes into a single 16-bit, big-endian value
69 | 	header := uint16(first)<<8 | uint16(second)
70 | 
71 | 	// Check if the header is in the map of valid headers
72 | 	_, isValid := validHeaders[header]
73 | 	return isValid
74 | }
```

zstd.go
```
1 | package archives
2 | 
3 | import (
4 | 	"bytes"
5 | 	"context"
6 | 	"io"
7 | 	"strings"
8 | 
9 | 	"github.com/klauspost/compress/zstd"
10 | )
11 | 
12 | func init() {
13 | 	RegisterFormat(Zstd{})
14 | }
15 | 
16 | // Zstd facilitates Zstandard compression.
17 | type Zstd struct {
18 | 	EncoderOptions []zstd.EOption
19 | 	DecoderOptions []zstd.DOption
20 | }
21 | 
22 | func (Zstd) Extension() string { return ".zst" }
23 | func (Zstd) MediaType() string { return "application/zstd" }
24 | 
25 | func (zs Zstd) Match(_ context.Context, filename string, stream io.Reader) (MatchResult, error) {
26 | 	var mr MatchResult
27 | 
28 | 	// match filename
29 | 	if strings.Contains(strings.ToLower(filename), zs.Extension()) {
30 | 		mr.ByName = true
31 | 	}
32 | 
33 | 	// match file header
34 | 	buf, err := readAtMost(stream, len(zstdHeader))
35 | 	if err != nil {
36 | 		return mr, err
37 | 	}
38 | 	mr.ByStream = bytes.Equal(buf, zstdHeader)
39 | 
40 | 	return mr, nil
41 | }
42 | 
43 | func (zs Zstd) OpenWriter(w io.Writer) (io.WriteCloser, error) {
44 | 	return zstd.NewWriter(w, zs.EncoderOptions...)
45 | }
46 | 
47 | func (zs Zstd) OpenReader(r io.Reader) (io.ReadCloser, error) {
48 | 	zr, err := zstd.NewReader(r, zs.DecoderOptions...)
49 | 	if err != nil {
50 | 		return nil, err
51 | 	}
52 | 	return errorCloser{zr}, nil
53 | }
54 | 
55 | type errorCloser struct {
56 | 	*zstd.Decoder
57 | }
58 | 
59 | func (ec errorCloser) Close() error {
60 | 	ec.Decoder.Close()
61 | 	return nil
62 | }
63 | 
64 | // magic number at the beginning of Zstandard files
65 | // https://github.com/facebook/zstd/blob/6211bfee5ec24dc825c11751c33aa31d618b5f10/doc/zstd_compression_format.md
66 | var zstdHeader = []byte{0x28, 0xb5, 0x2f, 0xfd}
```

.github/FUNDING.yml
```
1 | # These are supported funding model platforms
2 | 
3 | github: [mholt] # Replace with up to 4 GitHub Sponsors-enabled usernames e.g., [user1, user2]
4 | patreon: # Replace with a single Patreon username
5 | open_collective: # Replace with a single Open Collective username
6 | ko_fi: # Replace with a single Ko-fi username
7 | tidelift: # Replace with a single Tidelift platform-name/package-name e.g., npm/babel
8 | community_bridge: # Replace with a single Community Bridge project-name e.g., cloud-foundry
9 | liberapay: # Replace with a single Liberapay username
10 | issuehunt: # Replace with a single IssueHunt username
11 | otechie: # Replace with a single Otechie username
12 | custom: # Replace with up to 4 custom sponsorship URLs e.g., ['link1', 'link2']
```

.github/ISSUE_TEMPLATE/bug_report.md
```
1 | ---
2 | name: Bug report
3 | about: For behaviors which violate documentation or cause incorrect results
4 | title: ''
5 | labels: ''
6 | assignees: ''
7 | 
8 | ---
9 | 
10 | <!--
11 | This template is for bug reports! (If your issue doesn't fit this template, it's probably a feature request instead.)
12 | To fill out this template, simply replace these comments with your answers.
13 | Please do not skip questions; this will slow down the resolution process.
14 | -->
15 | 
16 | ## What version of the package or command are you using?
17 | <!-- A commit sha or tag is fine -->
18 | 
19 | 
20 | ## What are you trying to do?
21 | <!-- Please describe clearly what you are trying to do thoroughly enough so that a reader with no context can repeat the same process. -->
22 | 
23 | 
24 | ## What steps did you take?
25 | <!-- Explain exactly how we can reproduce this bug; attach sample archive files if relevant -->
26 | 
27 | 
28 | ## What did you expect to happen, and what actually happened instead?
29 | <!-- Please make it clear what the bug actually is -->
30 | 
31 | 
32 | ## How do you think this should be fixed?
33 | <!-- Being specific by linking to lines of code and even suggesting changes will yield fastest resolution -->
34 | 
35 | 
36 | ## Please link to any related issues, pull requests, and/or discussion
37 | <!-- This will help add crucial context to your report -->
38 | 
39 | 
40 | ## Bonus: What do you use this package for, and do you have any other suggestions or feedback?
41 | <!-- We'd like to know! -->
```

.github/ISSUE_TEMPLATE/generic-feature-request.md
```
1 | ---
2 | name: Generic feature request
3 | about: Suggest an idea for this project
4 | title: ''
5 | labels: feature request
6 | assignees: ''
7 | 
8 | ---
9 | 
10 | <!--
11 | This issue template is for feature requests! If you are reporting a bug instead, please switch templates.
12 | To fill this out, simply replace these comments with your answers.
13 | -->
14 | 
15 | ## What would you like to have changed?
16 | <!-- Describe the feature or enhancement you are requesting -->
17 | 
18 | 
19 | ## Why is this feature a useful, necessary, and/or important addition to this project?
20 | <!-- Please justify why this change adds value to the project, considering the added maintenance burden and complexity the change introduces -->
21 | 
22 | 
23 | ## What alternatives are there, or what are you doing in the meantime to work around the lack of this feature?
24 | <!-- We want to get an idea of what is being done in practice, or how other projects support your feature -->
25 | 
26 | 
27 | ## Please link to any relevant issues, pull requests, or other discussions.
28 | <!-- This adds crucial context to your feature request and can speed things up -->
```

.github/ISSUE_TEMPLATE/new-format-request.md
```
1 | ---
2 | name: New format request
3 | about: Request a new archival or compression format
4 | title: ''
5 | labels: ''
6 | assignees: ''
7 | 
8 | ---
9 | 
10 | <!--
11 | This template is specifically for adding support for a new archive or compression format to the library. Please, precisely one format per issue.
12 | To fill this out, replace these comments with your answers or add your answers after the comments.
13 | -->
14 | 
15 | ## Introduce the format you are requesting.
16 | <!-- What is it called, what is it used for, etc? Some background information. -->
17 | 
18 | 
19 | 
20 | ## What do YOU use this format for?
21 | <!-- We want to know YOUR specific use cases; why do YOU need this format? -->
22 | 
23 | 
24 | 
25 | ## What is the format's conventional file extension(s)?
26 | <!-- Don't overthink this one, it's a simple question. -->
27 | 
28 | 
29 | 
30 | ## What is the format's typical header bytes?
31 | <!-- Usually a file format starts with predictable bytes to determine what it is. -->
32 | 
33 | 
34 | 
35 | ## What is the format's MIME type?
36 | <!-- Also known as media type or, in HTTP terms, Content-Type. -->
37 | 
38 | 
39 | 
40 | ## Please link to the format's formal or official specification(s).
41 | <!-- If there isn't a formal spec, link to the most official documentation for the format. Note that unstandardized formats are less likely to be added unless it is in high-enough demand. -->
42 | 
43 | 
44 | 
45 | ## Which Go libraries could be used to implement this format?
46 | <!-- This project itself does not actually implement low-level format reading and writing algorithms, so link to pure-Go libraries that do. Dependencies that use cgo or invoke external commands are not eligible for this project. -->
47 | 
```

.github/workflows/macos-latest.yml
```
1 | name: Mac
2 | 
3 | on: [push, pull_request]
4 | 
5 | jobs:
6 | 
7 |   build-and-test:
8 |   
9 |     strategy:
10 |       matrix:
11 |         go-version: [1.23]
12 |     runs-on: macos-latest
13 |     steps:
14 |     - name: Install Go
15 |       uses: actions/setup-go@v5
16 |       with:
17 |         go-version: ${{ matrix.go-version }}
18 | 
19 |     - name: Checkout code
20 |       uses: actions/checkout@v4
21 | 
22 |     - name: Test
23 |       run: go test -v -race ./...
```

.github/workflows/ubuntu-latest.yml
```
1 | name: Linux
2 | 
3 | on: [push, pull_request]
4 | 
5 | jobs:
6 | 
7 |   build-and-test:
8 |   
9 |     strategy:
10 |       matrix:
11 |         go-version: [1.23]
12 |     runs-on: ubuntu-latest
13 |     steps:
14 |     - name: Install Go
15 |       uses: actions/setup-go@v5
16 |       with:
17 |         go-version: ${{ matrix.go-version }}
18 | 
19 |     - name: Checkout code
20 |       uses: actions/checkout@v4
21 | 
22 |     - name: Test
23 |       run: go test -v -race ./...
```

.github/workflows/windows-latest.yml
```
1 | name: Windows
2 | 
3 | on: [push, pull_request]
4 | 
5 | jobs:
6 | 
7 |   build-and-test:
8 |   
9 |     strategy:
10 |       matrix:
11 |         go-version: [1.23]
12 |     runs-on: windows-latest
13 |     steps:
14 |     - name: Install Go
15 |       uses: actions/setup-go@v5
16 |       with:
17 |         go-version: ${{ matrix.go-version }}
18 | 
19 |     - name: Checkout code
20 |       uses: actions/checkout@v4
21 | 
22 |     - name: Test
23 |       run: go test -v -race ./...
```

</current_codebase>
