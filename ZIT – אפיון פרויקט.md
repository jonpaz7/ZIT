# ZIT – אפיון פרויקט

מערכת בקרת גרסאות מבוזרת בסגנון Git, שבה האובייקטים מפוזרים בין קליינטים בסגנון טורנט.

## 1. מטרה

לאפשר לעבוד עם ריפו גדול בלי להוריד אותו כולו. כל קליינט מחזיק חלק מהאובייקטים, ומי שצריך קובץ מוריד אותו ישירות מקליינט אחר שמחזיק בו, ומאמת אותו מול ה-hash.

**הגדרת הצלחה ל-MVP:** שלושה תהליכים (tracker ושני קליינטים) על מחשב אחד או יותר. קליינט A יוצר ריפו ועושה commit. קליינט B עושה `checkout` ומקבל רק קבצים נבחרים, שיורדים ישירות מ-A.

## 2. החלטות מרכזיות (כדי לא להתלבט)

| נושא | החלטה | סיבה |
| --- | --- | --- |
| שפה | Go | ספריית רשת מובנית, קל לכתוב שרתים, בינארי יחיד |
| Hash | SHA-256 | פשוט, ספריה סטנדרטית |
| גילוי peers | Tracker מרכזי (HTTP) | DHT הוא שדרוג עתידי, לא חלק מה-MVP |
| תקשורת בין peers | HTTP | הכי קל לדבג, אפשר לבדוק עם curl |
| חתימות | Ed25519, מפתח בעלים יחיד | ספריה סטנדרטית ב-Go, מונע זיוף refs |
| Merge | לא נתמך | מסבך מאוד, לא נדרש לבסיס |
| קבצים גדולים / chunking | לא נתמך ב-MVP | blob = קובץ שלם |

## 3. ארכיטקטורה

```
   ┌────────────┐          ┌────────────┐
   │ קליינט A   │          │ קליינט B   │
   │ CLI + Peer │◄────────►│ CLI + Peer │
   └─────┬──────┘  objects └─────┬──────┘
         │                       │
         │  announce / lookup    │
         └───────►┌────────┐◄────┘
                  │Tracker │
                  │ refs + │
                  │providers│
                  └────────┘
```

שלושה רכיבים:

1. **CLI** – הפקודות שהמשתמש מריץ.
2. **Peer server** – שרת HTTP קטן בכל קליינט, מגיש את האובייקטים המקומיים לאחרים (`zit serve`).
3. **Tracker** – שרת HTTP יחיד. מחזיק שני דברים: מי מחזיק איזה אובייקט, ומה ה-refs של כל ריפו.

## 4. מבנה נתונים

### 4.1 אובייקטים

שלושה סוגים, כמו ב-Git. כל אובייקט נשמר כ-`<type> <size>\0<content>`, נדחס ב-zlib, ושמו הוא ה-SHA-256 של התוכן הלא דחוס.

- **blob** – תוכן קובץ.
- **tree** – רשימת רשומות, כל אחת: `mode name hash`, ממוינת לפי שם. תיקיות הן tree בתוך tree.
- **commit** – שדות: `tree`, `parent` (אפס או אחד), `author`, `timestamp`, והודעה.

### 4.2 מבנה תיקיית `.zit/`

```
.zit/
  config.json        # כתובת tracker, כתובת peer server, repo id
  HEAD               # "ref: refs/heads/main" או hash ישיר
  index.json         # staging area: path → blob hash
  keys/
    private.key
    public.key
  objects/
    ab/cdef...       # אובייקטים מקומיים
  refs/
    heads/main       # hash של commit
```

### 4.3 Staging area (index)

קובץ JSON פשוט: `{ "src/main.go": "hash...", ... }`. הפקודה `add` מעדכנת אותו, ו-`commit` בונה ממנו עץ.

## 5. פרוטוקול רשת

### 5.1 Tracker API

| Endpoint | תיאור |
| --- | --- |
| `POST /announce` | גוף: `{hashes: [...], peer: "host:port"}`. רושם שה-peer מחזיק את האובייקטים. |
| `GET /providers/{hash}` | מחזיר רשימת peers שמחזיקים את ה-hash. |
| `GET /refs/{repoId}` | מחזיר את כל ה-refs של הריפו עם החתימות. |
| `PUT /refs/{repoId}/{ref}` | מעדכן ref. גוף: `{commit, signature}`. ה-tracker מאמת חתימה מול המפתח הציבורי של הריפו. |

ב-MVP ה-tracker שומר הכול בזיכרון עם dump ל-JSON בדיסק. אין צורך בבסיס נתונים.

**Peers שנעלמו:** כל announce תקף 10 דקות, ו-peer מחדש אותו בכל דקה (heartbeat). כך ה-tracker לא מחזיק peers מתים.

### 5.2 Peer API

| Endpoint | תיאור |
| --- | --- |
| `GET /objects/{hash}` | מחזיר את האובייקט (בפורמט דחוס), או 404. |
| `GET /has/{hash}` | בדיקה מהירה אם יש. |

### 5.3 כלל אימות

כל אובייקט שמגיע מהרשת נבדק: מחשבים hash על התוכן ומשווים. אם לא תואם, זורקים ומנסים peer אחר. לעולם לא סומכים על peer.

## 6. הפקודות

### Git בסיסי (חייב ב-MVP)

| פקודה | התנהגות |
| --- | --- |
| `zit init` | יוצר `.zit/`, מייצר זוג מפתחות, כותב config. |
| `zit add <path...>` | יוצר blobs ושומר ב-objects, מעדכן index. תומך בתיקיות. |
| `zit commit -m "msg"` | בונה trees מה-index, יוצר commit, מעדכן את ה-branch הנוכחי. |
| `zit branch [name]` | בלי ארגומנט: מציג branches. עם שם: יוצר branch חדש ב-HEAD. |
| `zit checkout <branch\|hash>` | מחליף branch ומעדכן את תיקיית העבודה (ראה סעיף 7). |
| `zit log` | הולך על שרשרת ה-parents ומדפיס commits. |
| `zit status` | משווה: working dir מול index מול HEAD. |

### פקודות הרשת (מה שמייחד את ZIT)

| פקודה | התנהגות |
| --- | --- |
| `zit serve` | מפעיל את ה-peer server ומבצע announce ל-tracker על כל האובייקטים המקומיים, עם heartbeat. |
| `zit publish` | דוחף את ה-branch הנוכחי: announce לאובייקטים, וחותם ומעדכן את ה-ref ב-tracker. |
| `zit clone <tracker-url> <repoId>` | מוריד **רק** refs, commits ו-trees. לא מוריד blobs. |
| `zit fetch` | מעדכן refs מה-tracker ומוריד commits ו-trees חדשים. |
| `zit checkout <branch> -- <path>` | Sparse: מוריד רק את ה-blobs שבתת-העץ המבוקש. |

## 7. Lazy checkout (הלב של הפרויקט)

עקרון: **commits ו-trees יורדים תמיד, blobs יורדים רק לפי דרישה.**

אלגוריתם `checkout <branch> [-- paths]`:

1. קורא את ה-ref ומשיג את ה-commit דרך `NetworkStore.Get`.
2. עובר רקורסיבית על ה-tree. כל tree מושג דרך `NetworkStore.Get` (קטן, יורד תמיד).
3. לכל רשומת blob שנמצאת בתוך ה-paths המבוקשים (או בכולם אם לא צוין path): מושג דרך `NetworkStore.Get` וכותב לדיסק.
4. blobs שלא בתוך ה-paths לא יורדים. נשמר ב-`.zit/sparse.json` מה נבחר, כדי ש-`status` לא יתייחס לקבצים החסרים כאל מחוקים.

**הורדה מקבילית:** לוקחים רשימת כל ה-blobs הנדרשים ומורידים עם worker pool (למשל 8 goroutines). זה מה שנותן את תחושת הטורנט.

## 8. הממשק המרכזי: ObjectStore

כל הפקודות עובדות מול ממשק אחד ולא יודעות אם האובייקט מקומי או מרוחק:

```go
type ObjectStore interface {
    Has(hash string) bool
    Get(hash string) ([]byte, error)
    Put(obj []byte) (hash string, err error)
}

type LocalStore struct { dir string }

type NetworkStore struct {
    local   *LocalStore
    tracker *TrackerClient
}

func (n *NetworkStore) Get(hash string) ([]byte, error) {
    if n.local.Has(hash) {
        return n.local.Get(hash)
    }
    peers := n.tracker.Providers(hash)
    for _, p := range shuffle(peers) {
        data, err := p.Fetch(hash)
        if err == nil && sha256Hex(data) == hash {
            n.local.Put(data)   // cache + אפשר להפוך ל-seeder
            return data, nil
        }
    }
    return nil, ErrNotFound
}
```

## 9. אבטחה (ברמת MVP)

- **שלמות תוכן:** מובטחת על ידי ה-hash, לכן אין צורך להאמין לשום peer.
- **שלמות refs:** כל עדכון ref חתום ב-Ed25519 על ידי בעל הריפו. ה-tracker ומי שעושה fetch מאמתים את החתימה מול המפתח הציבורי שמוגדר ב-repoId (repoId = hash של המפתח הציבורי).
- **מחוץ לתחום:** הצפנה, הרשאות קריאה לפי משתמש, ריבוי maintainers, הגנה מפני tracker זדוני שמסתיר עדכונים.

## 10. מבנה הקוד

```
zit/
  cmd/zit/main.go          # CLI (cobra או flag רגיל)
  cmd/zit-tracker/main.go  # שרת ה-tracker
  internal/
    object/    # blob, tree, commit: סידור ופענוח
    store/     # ObjectStore, LocalStore, NetworkStore
    repo/      # init, index, refs, HEAD
    commands/  # add, commit, branch, checkout, log, status...
    peer/      # peer HTTP server + client
    tracker/   # tracker server + client
    crypto/    # מפתחות וחתימות
```

## 11. שלבי מימוש

כל שלב מסתיים בדבר שאפשר להריץ ולבדוק.

**שלב 1 – Git מקומי (כ-1–2 שבועות)** מימוש `object`, `LocalStore`, `init`, `add`, `commit`, `log`, `branch`, `checkout` (מלא, בלי רשת). *קריטריון:* ליצור ריפו, לעשות כמה commits על שני branches ולעבור ביניהם, והקבצים משתנים נכון.

**שלב 2 – Tracker ו-peer בסיסי (כ-1 שבוע)** מימוש tracker עם `announce` ו-`providers`, ו-`zit serve`. בדיקה עם curl שאפשר להוריד אובייקט מ-peer אחר. *קריטריון:* שני תהליכים; `curl` מ-B לאובייקט ב-A מצליח.

**שלב 3 – NetworkStore ו-clone (כ-1 שבוע)** מימוש `NetworkStore`, `publish`, `clone`, `fetch` (בלי חתימות עדיין, ref פשוט). *קריטריון:* B עושה `clone` ומקבל `log` מלא בלי שירדו blobs.

**שלב 4 – Lazy ו-sparse checkout (כ-1 שבוע)** `checkout -- path`, הורדה מקבילית, `sparse.json`. *קריטריון:* בריפו עם תיקיות `a/` ו-`b/`, B עושה checkout רק ל-`a/` ולא נמצא אצלו שום blob מ-`b/` (בדיקה ב-`.zit/objects`).

**שלב 5 – חתימות ו-polish (כ-1 שבוע)** חתימה על refs, אימות ב-tracker וב-fetch, heartbeat, הודעות שגיאה סבירות, `status`. *קריטריון:* עדכון ref לא חתום נדחה.

## 12. תרחיש הדגמה

```bash
# מכונה 1
zit-tracker --port 9000
zit init && zit add . && zit commit -m "first"
zit serve --tracker http://host:9000 &
zit publish

# מכונה 2
zit clone http://host:9000 <repoId>
zit log                      # עובד, בלי blobs
zit checkout main -- docs/   # מוריד רק את docs/ מ-peer 1

# מכונה 3 (אחרי שמכונה 2 עשתה serve)
# הורדה של docs/ יכולה להגיע מכל אחת משתי המכונות
```

## 13. מחוץ ל-MVP (שדרוגים אפשריים)

- DHT (libp2p) במקום tracker מרכזי.
- Merge ו-rebase.
- Chunking של קבצים גדולים עם Merkle tree והורדה מקבילית ממספר peers לאותו קובץ.
- Replication factor: דרישה שכל אובייקט יוחזק אצל לפחות k peers, ו-pinning.
- FUSE: הקובץ יורד ברגע שפותחים אותו.
- Packfiles ו-delta compression.
- ריבוי maintainers ו-ACL.

## 14. בדיקות

- **Unit:** סידור/פענוח של אובייקטים, חישוב hash, בניית tree מ-index, אימות חתימות.
- **אינטגרציה:** סקריפט שמרים tracker ושני peers על localhost ומריץ את תרחיש ההדגמה אוטומטית.
- **בדיקת אבטחה בסיסית:** peer שמחזיר תוכן שגוי חייב להידחות, והקליינט עובר ל-peer אחר.
