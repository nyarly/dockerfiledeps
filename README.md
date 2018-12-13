# dockerfiledeps

This is a tiny tool to support building Docker images with a Makefile.

Create your project repo with directories for each target image,
where the directory name is the repo name of images.

Where image `foo` decends from image `bar`,
start it with `FROM my.registry.net/project/bar:local`.

Run `dockerfiledeps -emit-driver` and start a `Makefile`
like
```
REGISTRY_HOST := my.registry.net
REPOSITORY_NAME := project
include driver.mk

push-all: push-foo
  @echo done
```

Then you can `make push-all` and get the Make goodness of minimal build times.
You can add rules for files in the various image directories to speed builds, as well.

## How It Works

_or, haven't we heard this one before?_

The new thing in dockerfiledeps is that it uses
the Docker code to parse the Dockerfile
and build Make dependencies on precise files.
If you `ADD .` you won't get much benefit,
but if you're specific about
the files you copy into the image,
your build cycles will see the benefit.
Additionally,
dockerfiledeps uses proxy targets to record
the freshness of built images,
so you get the niceness of not rebuilding images that haven't really changed.
