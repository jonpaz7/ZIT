# ZIT – Distributed Version Control with BitTorrent-like Object Storage
### מערכת בקרת גרסאות מבוזרת עם אחסון אובייקטים בסגנון טורנט

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux%20%7C%20macOS-lightgrey)](#)
[![Tests](https://img.shields.io/badge/Tests-Passing-brightgreen.svg)](#)

מערכת בקרת גרסאות בסגנון Git, המאפשרת לעבוד מול מאגרים ענקיים מבלי להוריד את כל ה-blobs מראש (**Lazy & Sparse Checkout**). אובייקטי המטא-דאטה (commits, trees, refs) יורדים תמיד, בעוד קובצי הנתונים (blobs) יורדים ישירות מעמיתים (peers) ברשת לפי דרישה.

📄 **מדריך מקיף ב-PDF עם אנלוגיות וסיפור משתמש:** [ZIT_Project_Guide.pdf](ZIT_Project_Guide.pdf)

---

## ⚡ התקנה מהירה (הפעלת הפקודות מכל מקום בלי נתיבים!)

### ב-Windows (PowerShell):
פתח PowerShell בתיקיית הפרויקט והרץ:
```powershell
powershell -ExecutionPolicy Bypass -File .\install.ps1
```
> **מה הסקריפט עושה?** מעתיק את `zit.exe` ו-`zit-tracker.exe` לתיקיית התוכנות שלך ומוסיף אותה ישירות ל-`PATH`.  
> מעכשיו, בכל חלון טרמינל חדש, תוכלו פשוט לרשום `zit` ו-`zit-tracker` מכל מקום בדיוק כמו `git`!

### ב-Linux / macOS:
```bash
chmod +x install.sh
./install.sh
```

---

## 🚀 מדריך שימוש מהיר (Quickstart למשתמש ולמורה)

נניח שרוצים לבדוק את המערכת בין שני משתמשים (או בשני חלונות על אותו מחשב):

### שלב 1: מפעילים את המרכזייה (הטראקר)
הטראקר מנהל את הרישום של מי מחזיק איזה קובץ. נפעיל אותו בחלון טרמינל:
```bash
zit-tracker --port 9000
```
*(משאירים את החלון הזה פתוח ברקע).*

### שלב 2: משתמש א' (יוצר הפרויקט)
בחלון טרמינל חדש, נכנסים לתיקייה שרוצים לפתח בה:
```bash
# יוצרים תיקייה לפרויקט
mkdir my-project
cd my-project

# מאתחלים פרויקט ZIT ומחברים לטראקר
zit init --tracker http://127.0.0.1:9000 --peer 127.0.0.1:8001
```
הפקודה תדפיס: `Repo ID: <קוד_ארוך>` (לדוגמה `a27bb73...`). שמרו את הקוד הזה!

כעת ניצור קבצים, נוסיף ונשמור:
```bash
mkdir docs src
echo "מדריך למשתמש" > docs/guide.txt
echo "קוד התוכנה" > src/app.go

# שומרים גרסה (commit)
zit add docs src
zit commit -m "גרסה ראשונה של הפרויקט"

# מפעילים את שרת השיתוף (שיתוף עמיתים ברקע)
zit serve --port 8001 &

# מפרסמים את הגרסה לטראקר
zit publish
```

### שלב 3: משתמש ב' (החבר / מחשב אחר)
בחלון טרמינל נפרד:
```bash
# משכפלים את הפרויקט (רק מטא-דאטה, 0 קבצים כבדים יורדים בשלב זה!)
zit clone http://127.0.0.1:9000 <ה-RepoID_משלב_2> friend-repo

cd friend-repo

# צפייה בהיסטוריית הגרסאות (עובד מיד ללא הורדת קבצים!)
zit log

# הורדה סלקטיבית (Sparse Checkout) - מורידים אך ורק את תיקיית docs/:
zit checkout main -- docs/
```
> **הקסם של ZIT:** תוך שבריר שנייה, רק הקובץ `docs/guide.txt` יורד ישירות ממשתמש א', בעוד שתיקיית `src` כלל לא תופסת מקום בדיסק!

---

## 📖 מילון הפקודות המלא

### פקודות ניהול גרסאות (Local Git)
| פקודה | תיאור |
| :--- | :--- |
| `zit init [dir]` | מאתחל מאגר ZIT חדש, מייצר מפתחות Ed25519 ומחשב Repo ID. |
| `zit add <path...>` | מוסיף קבצים או תיקיות ל-Staging area לקראת שמירה. |
| `zit commit -m "הודעה"` | שומר גרסה חדשה (יוצר commit ובונה עץ אובייקטים). |
| `zit branch [name]` | ללא שם: מציג את רשימת הענפים. עם שם: יוצר ענף חדש. |
| `zit log` | מציג את היסטוריית הגרסאות והשינויים בסדר כרונולוגי. |
| `zit status` | מציג את מצב הקבצים (Staged, Unstaged, Untracked), ומתחשב ב-Sparse. |
| `zit checkout <branch>` | מעבר בין ענפים והורדת כל הקבצים של הגרסה. |
| `zit checkout <branch> -- <paths>` | **Sparse Checkout:** הורדה סלקטיבית רק של התיקיות שנבחרו. |

### פקודות רשת ושיתוף (P2P & Tracker)
| פקודה | תיאור |
| :--- | :--- |
| `zit-tracker [--port 9000]` | מפעיל את שרת הטראקר המרכזי שמנהל את רישום העמיתים. |
| `zit serve [--port 8001]` | מפעיל שרת עמית מקומי שמשתף (seeds) את הקבצים המקומיים לאחרים. |
| `zit publish` | חותם קריפטוגרפית על הגרסה, מכריז על האובייקטים ומעדכן את הטראקר. |
| `zit clone <tracker> <repoId>` | **Lazy Clone:** משכפל מטא-דאטה ועצים מהרשת ללא הורדת blobs. |
| `zit fetch` | מושך עדכוני גרסאות ועצים חדשים מהטראקר (הקבצים עצמם נשארים עצלים). |

---

## 🧪 בדיקות אוטומטיות (E2E Demo)
לצורך בדיקת אינטגרציה מלאה, הפרויקט כולל סקריפט הדגמה אוטומטי שמריץ טראקר ו-3 קליינטים עצמאיים:
```powershell
powershell -ExecutionPolicy Bypass -File .\demo.ps1
```
הסקריפט בודק:
1. אתחול טראקר וקליינט א'.
2. שכפול עצל אצל קליינט ב' (וידוא שירדו 0 blobs).
3. הורדה סלקטיבית של תיקיית מסמכים בלבד.
4. שיתוף עמיתים: קליינט א' נכבה, וקליינט ג' מוריד את הקבצים ישירות מקליינט ב'.

---

## 🏗️ מבנה הקוד
```
ZIT/
├── cmd/
│   ├── zit/              # נקודת הכניסה של ה-CLI
│   └── zit-tracker/      # שרת הטראקר
├── internal/
│   ├── crypto/           # Ed25519 חתימות, מפתחות ו-SHA-256
│   ├── object/           # טיפול ב-blob, tree, commit ודחיסת zlib
│   ├── store/            # LocalStore ו-NetworkStore מבוזר
│   ├── repo/             # ניהול תיקיית .zit, config, HEAD ו-sparse
│   ├── commands/         # מימוש כל פקודות המשתמש
│   ├── peer/             # שרת ולקוח P2P להעברת קבצים בין עמיתים
│   └── tracker/          # שרת ולקוח Tracker
├── install.ps1           # סקריפט התקנה מהיר ל-Windows
├── install.sh            # סקריפט התקנה ל-Linux/macOS
├── demo.ps1              # בדיקת E2E אוטומטית מלאה
└── ZIT_Project_Guide.pdf # מדריך גרפי מפורט ומעוצב
```

---

## 📜 רישיון
פרויקט זה מופץ תחת רישיון MIT.
