# ldapAuth filter

The `ldapAuth` filter can be used to validate basic auth requests
against an LDAP server. After successful authentication the username
is placed in the `X-Authenticated-User` header.

## Filter Config

The `ldapAuth` filter expects one parameter: the realm as string, e.g.
`ldapAuth("My secret place")`

## Module Config

### Server Config

* uri - the LDAP server to use as URI, e.g. `ldaps://ldap.example.com:1636`,
 default port for `ldap://` is `389`, for `ldaps://` is `636`.
* insecure - boolean, OPTIONAL, default `false` - when true, does not check
 the validity of the SSL certificate when connecting via SSL

### Searching For The User

When the users do not have a fixed DN pattern, the user's DN must be searched
before the authentication can be done. For this the following settings must
be set:

* base - the base DN for searching
* user - DN used for searching the user's DN
* pass - pass for the searching user
* filter - the user filter, must contain exactly one `%s`. The filter passed to
 the search is constructed like `fmt.Sprintf(f.Filter, ldap.EscapeFilter(user))`
* scope - search scope - can be `sub`, `subtree`, `one`, `single` or (even if not
 useful at all) `base` - OPTIONAL, defaults to `sub`.

### User Templates

If all users have the same DN pattern like
`uid=LOGINNAME,ou=people,dc=example,dc=com` and use the `LOGINNAME` to login, the
parameter 

* template 

can be used to directly go to authentication phase. The `template` must contain
exactly one `%s` where the requested user name will be, e.g. for the above
pattern it should be `uid=%s,ou=people,dc=example,dc=com`.
