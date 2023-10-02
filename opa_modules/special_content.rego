package specialContent

import future.keywords.if

default message := ""

message := "Skipped content due to access mismatch" if {
    input.EditorialDesk == "/FT/Professional/Central Banking"
}

message := "Content UUID was not found. Message will be skipped." if {
    input.UUID == ""
}