# Kanban

The language of the kanban app. The app uses the Vibrant Gio library's
terms as the library defines them; this file holds the terms of the app.

## Language

### Board

The tasks of one markdown file, shown as columns side by side. Each
level-2 heading of the file is a column, in file order. The app keeps
every other line of the file as it is and does not show it.

### Column

A column is the library's column, and in this app it holds the cards of
the tasks under one level-2 heading of the board's file.

### Card

A card is the library's card, and in this app it holds the title of one
task.

### Task

One task list item under a column's heading that is not indented: its
title, its state, and its notes. A task is complete when its box holds an
x, as in `- [x]`. The lines indented under the item are its notes.
