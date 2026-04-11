Study as part of the guided project https://www.boot.dev/courses/build-blog-aggregator-golang

PREREQUISITES
In order to run the application you will need to install Postgres and Go present
Postgresql - https://www.postgresql.org/download/   |  https://neon.com/postgresql/postgresql-getting-started
Go - https://go.dev/doc/install

INSTALLATION
In order to install the application from within the Gator directiry run 'go install'

CONFIGURATION
Connect to your psql instance and make a gator database
`CREATE DATABASE gator;`
Connect to the database and update the credentials for the database
`\c gator`
`ALTER USER postgres PASSWORD 'myVeryImportantPassword';`

You can store your connection string and credetnials in an environment variable `psql_conn_string` or you can manually create a JSON file with the below layout in the user's home directory.
```
{
    "db_url":"postgres://postgres:myVeryImportantPassword@localhost:5432/gator?sslmode=disable",
    "current_user_name":""
}
```
If using a JSON file ensure that you leave "current_user_name" as an empty string `""` and to update your connection string

USAGE
The application can be run entering 'gator command name "arg1" "arg2"'
e.g. 
`gator <command> <arg> <arg>
`gator register "username"`
`gator addfeed "Hacker News" "https://news.ycombinator.com/rss"`

You will need to register users before they can add or follow any feed
Switching users can be done using the login command
addfeed will add new feeds while follow will allow the current user to follow an existing feed
all feeds interactions commands are run against the logged on user 
run `gator agg <interval>` with 10s for 10 seconds, 1m for 1 minute. Leave gator agg running to keep pulling any updated posts and you can interact in a separate window. Updates will be posted to stdout when running, this can be piped to logs where required. 
you can browse posts using `gator browse`
