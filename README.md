# gstash — Git Stash Manager

Kelola `git stash` dengan tampilan visual ala GitKraken, langsung di terminal.
Filter otomatis berdasarkan branch aktif, preview diff, apply/pop/drop/branch dari satu layar.

## Install

```bash
go install .        # binary 'gstash' ke $GOBIN/$GOPATH/bin
# atau
go build -o gstash .
```

## Cara pakai

Jalankan `gstash` tanpa argumen di dalam repo git:

```
↑/↓ move · tab filter · a apply · p pop · d drop · b branch · r refresh · q quit
```

Panel kiri: daftar stash (branch asal + pesan). Panel kanan: preview diff stash terpilih
(scroll dengan `pgup`/`pgdn`). `tab` mengganti filter *current branch* ↔ *all branches*.

## Perintah CLI

| Perintah | Fungsi |
|---|---|
| `gstash` / TUI | Browser interaktif |
| `gstash list [--all]` | Daftar stash, default difilter branch aktif |
| `gstash show <index>` | Diff lengkap stash |
| `gstash save [pesan] [-u]` | Buat stash (`-u` sertakan untracked); mencatat nama branch di pesan |
| `gstash apply <index>` | Terapkan stash tanpa menghapusnya |
| `gstash pop <index>` | Terapkan + hapus |
| `gstash drop <index> [\-y]` | Hapus stash (dengan konfirmasi) |
| `gstash branch <index> [nama]` | Buat branch baru dari stash |

## Asosiasi stash ↔ branch

Git tidak punya kolom branch di stash, jadi `gstash` memakai 3 lapis strategi:

1. **Exact** — git sendiri menulis `On main:`/`WIP on main:` di reflog stash; itu dipercaya penuh.
2. **Prefix** — stash buatan `gstash save` menyimpan `[branch:<nama>]` di pesan.
3. **Inferred (~)** — dicari dari parent commit stash via `git branch --contains`.
4. **Unknown (?)** — selalu tetap ditampilkan agar tidak ada stash yang "hilang".

Filter default hanya menampilkan stash milik branch aktif (+ yang unknown), supaya daftarmu bersih.
hello
