package specialContent

import future.keywords.if

default msg := ""

msg := "Skipped content due to access mismatch" if {
    input.EditorialDesk == "/FT/Professional/Central Banking"
}

msg := "Content UUID was not found. Message will be skipped." if {
    input.UUID == ""
}