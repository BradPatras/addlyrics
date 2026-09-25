# addlyrics
CLI tool for adding lyric metadata to music files.

#### Disclaimer
**Use this tool at your own risk**. This is a hobby project intended for my personal use and to learn more about golang. This tool has the capacity to screw up your mp3 files so i'd recommend making a backup of your files (in a separate directory) before running it.

## Get it

3 options:

**Download it** from the Releases tab, unzip it and run it like any other program

-- or --

**Install it** (assuming you have go installed)

```sh
go install github.com/BradPatras/addlyrics@latest
```

-- or --

**Build it yourself** (assuming you have go installed)

```sh
# after cloning this repo
cd addlyrics
go install
```

## Run it

```sh
# call it with -h to get the available commands
addlyrics -h

# an example usage would be:
addlyrics -t ~/Music/Muse/
# that will search that directory for mp3s, fetch lyrics for all found, and apply the lyric data to the files.
```

### Limited scope
I'm planning on only implementing my primary usecase: mp3 files with ID3v2.3 metadata.
