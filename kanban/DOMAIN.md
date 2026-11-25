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
title, whether it is completed, and its notes. A task is completed when
its task list item is checked. The lines indented under the item are its
notes.

### Change

A change to the board that the library's Undo reverses and Redo makes
again. The Edit menu names each change after "Undo" or "Redo": Add Card,
Delete Card, Mark as Completed, Mark as Not Completed, Edit Title, and
Move Card. Move to Column puts the card at the end of the column, where a
new card goes.

After an undo or a redo the selection stays on the card it marked. When
the change removes that card, nothing is selected. After an undo or a
redo of a move, the column that holds the selected card holds the
keyboard, when a column held it before. An open edit stays with its card
and closes when the card is gone.
