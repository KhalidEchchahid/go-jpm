# LaTeX deliverables

This folder contains the final report and presentation for the JPM Phase 2 “native engine” work.

## Build (PDF)

```bash
cd latex/report
pdflatex -interaction=nonstopmode report.tex
pdflatex -interaction=nonstopmode report.tex

cd ../slides
pdflatex -interaction=nonstopmode slides.tex
pdflatex -interaction=nonstopmode slides.tex
```

If you have `latexmk` installed:

```bash
cd latex/report
latexmk -pdf -interaction=nonstopmode report.tex

cd ../slides
latexmk -pdf -interaction=nonstopmode slides.tex
```
