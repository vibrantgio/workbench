// Package workbench is the module at the root of the repository that collects
// the applications.
//
// Each application here — feeds, iconbrowser, launcher, mindchat, sitedocs,
// themer, todos and vaultview — is a module of its own, built, tested and
// released on its own cadence. A nested module stands outside its parent by
// Go's own rules, so this module's packages are only the ones at the
// repository root: building it never builds an application, and tagging it
// never releases one.
//
// This module is where the launcher that opens those applications will live,
// so that the repository path names a program that can be fetched and run.
package workbench
