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
A column is drawn as a group titled with its name.

### Card

A card is the library's card, and in this app it holds the title of one
task.

### Task

One task list item under a column's heading that is not indented: its
title, whether it is complete, and its notes. A task is complete when its
task list item is checked. The lines indented under the item are its
notes.

### Change

A change to the board that [[DOMAIN#Undo]] reverses and Redo makes
again: adding a card, deleting a card, marking a card as completed or as
not completed, and editing a card's title. A move of a card is a change
too, once the app moves cards.

After an undo or a redo the selection stays on the card it marked. When
the change removes that card, nothing is selected. An open edit stays
with its card and closes when the card is gone.
