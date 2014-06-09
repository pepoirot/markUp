MarkUp: a tiny Markdown server
==============================

MarkUp is a tiny Markdown server that serves and renders Markdown files
in a directory (and its subdirectories).

Usages
------

To serve the Markdown documents from the current directory:

    $ ./markUp 

To serve the Markdown documents _only_ from the current directory:

    $ ./markUp --recursive=false

To serve the Markdown documents from a different directory:

    $ ./markUp --root=~/markdown-docs

To serve the Markdown documents on a port different from the default (8888):

    $ ./markUp --port=8000

To style the Markdown documents with a custom stylesheet:

    $ ./markUp --stylesheet=http://example.com/stylesheet.css

License
-------

BSD License, see the LICENSE.txt at the root of the repository.

Building from source
--------------------

Install [Go][1] and run:

    $ go build -o markUp

CSS stylesheet
--------------

Default stylesheet by [Andy Ferra][2].


[1]: http://golang.org/doc/install#install
[2]: https://gist.github.com/2554919/10ce87fe71b23216e3075d5648b8b9e56f7758e1
