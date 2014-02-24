/* This is an IRC BOT it joins the specified channel below
 * by the specified nickname and sits waiting for commands.
 *
 * It can accept votes for users up or down for silly
 * internet points, the best kind of points.
 * TODO - Make a simple timer to check for 1 vote an hour/half hour/ten minutes.
 * TODO - Make a simple timer to only allow for 1 help per minute.
 * TODO - Put votes in IRCBot struct.
 * TODO - Keep track of who voted for who.
 * TODO - Stop self voting.
 */
package main

import (
  "fmt"
  "bufio"
  "log"
  "net"
  "net/textproto"
  "strings"
  "strconv"
  "time"
)

var respReq *textproto.Reader


const (
  //Different commands the bot has
  CMD_VOTE_UP = "!voteUp"
  CMD_VOTE_DOWN = "!voteDown"
  CMD_HELP = "!help"
  CMD_VOTES = "!votes"

  //Different errors the bot can throw
  NO_USER_ERROR = "!No user specified!"
  USER_NOT_FOUND_ERROR = "!User not in channel!"

  //Different IRC reply numbers
  NAME_REPLY = "353"
)

//Tried to add most of the commands into the bot
//in order to keep from having too many globals.
//feel free to change these back.
type IRCBot struct {
  Server      string
  Port        string
  User        string
  Nick        string
  Channel     string
  Pass        string
  Votes       map[string]int
  Users       []string
  Incomming   chan string
  Connection  net.Conn
}

//create a new bot with the given server info
//currently static but will create prompt for any server
func createBot() *IRCBot {
  //serverPrompt()
  return &IRCBot {
    Server:     "irc.freenode.net",
    Port:       "6665",
    Nick:       "DeanBot",
    Channel:    "#orderdeck",
    Pass:       "",
    Connection: nil,
    Incomming:  make(chan string),
    Votes:      make(map[string]int),
    Users:      make([]string, 1),
    User:       "VoteBot",
    }
}

/*On creation of a bot prompts the user for correct server information
func severPrompt() {
}*/

//connects the bot to the server & puts the connection in the bot
func (bot *IRCBot) ServerConnect() (err error) {
  bot.Connection, err = net.Dial("tcp", bot.Server + ":" + bot.Port)
  if err != nil {
    log.Fatal("Unable to connect to the specified server", err)
    return err
  }
  log.Printf("Successfully connected to %s (%s)\n", bot.Server, bot.Connection.RemoteAddr())
  return nil
}

func (bot *IRCBot) listenToChannel() (err error) {
  var line string
  for {
    line, err = respReq.ReadLine()
    if err != nil {
      break
    }
    fmt.Printf("\033[93m%s\n", line)
    if(line != "") {
      bot.Incomming <- line
    }
  }
  return nil
}

//Starts the bot, initalizes votes to 0 & starts
//a constant loop reading in lines & waiting for commands.
func main() {
  bot := createBot()
  _ = bot.ServerConnect()
  bot.sendCommand("USER", bot.Nick, "8 *", bot.Nick)
  bot.sendCommand("NICK", bot.Nick)
  bot.sendCommand("JOIN", bot.Channel)
  defer bot.Connection.Close()

  reader := bufio.NewReader(bot.Connection)
  respReq = textproto.NewReader(reader)
  go bot.listenToChannel()
  //specialListen <- "stop"
  var waitStart time.Time
  wait := false
  for {
    line, available := <-bot.Incomming
    if(!available) {
      break;
    }
    if strings.Contains(line, "PING") {
      bot.sendCommand("PONG", "")
      time.Sleep(2 * time.Second)
    }
    if strings.Contains(line, NAME_REPLY) {
      bot.setUsers(line)
    }
    if(!wait || (time.Now().Sub(waitStart).Seconds()>2)) {
      wait=false
      //Sends the PONG command in order not to get kicked from the server
      if strings.Contains(line, bot.Channel) && strings.Contains(line, CMD_VOTE_UP) {
        err := bot.voteUp(line)
        if err != nil {
          break
        }
        waitStart = time.Now()
        wait = true;
      }
      if strings.Contains(line, bot.Channel) && strings.Contains(line, CMD_VOTE_DOWN) {
        err := bot.voteDown(line)
        if err != nil {
          break
        }
        waitStart = time.Now()
        wait = true;
      }
      if strings.Contains(line, bot.Channel) && strings.Contains(line, CMD_HELP) {
        bot.help()
        waitStart = time.Now()
        wait = true;
      }
      if strings.Contains(line, bot.Channel) && strings.Contains(line, CMD_VOTES) {
        bot.getVotes()
        waitStart = time.Now()
        wait = true;
      }
    }
  }
}

//Sends the specified command along with any parameters needed as well
func (bot *IRCBot) sendCommand(command string, parameters ...string) {
  msg := strings.Join(parameters, " ")
  cmd := fmt.Sprintf("%s %s", command, msg)
  fmt.Fprintf(bot.Connection, strings.Join([]string{cmd, "\n"}, ""));
  fmt.Printf(strings.Join([]string{cmd, "\n"}, ""));
}

//Uses the sendCommand to PRIVMSG the channel 
//as well as sends a []string message
func (bot *IRCBot) sendMessage(recipient string, message ...string) {
  msg := fmt.Sprintf("%s : %s", recipient, strings.Join(message, " "))
  bot.sendCommand("PRIVMSG", msg)
}

//Votes the user up & saves it to votes.
//If the user isnt in the channel or isnt speicified it will complain
func (bot *IRCBot) voteUp(line string) error {
  commandLine := line[strings.Index(line, CMD_VOTE_UP):len(line)]
  bot.sendCommand("NAMES", bot.Channel)

  if strings.Index(line, CMD_VOTE_UP) != -1 {
    commands := strings.Split(commandLine, " ")
    if len(commands) ==1 {
      bot.sendMessage(bot.Channel, "Error: ", NO_USER_ERROR)
    } else {
      voteUser := commands[1]
      contained, err := bot.inChannel(voteUser)
      if err != nil {
        return err
      }
      if contained {
        bot.Votes[voteUser] += 1
        bot.sendMessage(bot.Channel, "Upvoted:", voteUser)
      } else {
        bot.sendMessage(bot.Channel, "Error:", USER_NOT_FOUND_ERROR)
      }
    }
  }
  return nil
}

//Votes the user down & saves it to votes.
//If the user isnt in the channel or isnt speicified it will complain
func (bot *IRCBot) voteDown(line string) error {
  commandLine := line[strings.Index(line, CMD_VOTE_DOWN):len(line)]
  bot.sendCommand("NAMES", bot.Channel)
  if strings.Index(line, CMD_VOTE_DOWN) != -1 {
    commands := strings.Split(commandLine, " ")
    if len(commands) ==1 {
      bot.sendMessage(bot.Channel, "Error:", NO_USER_ERROR)
    } else {
      voteUser := commands[1]
      contained, err := bot.inChannel(voteUser)
      if err != nil {
        return err
      }
      if contained {
        bot.Votes[voteUser] -= 1
        bot.sendMessage(bot.Channel, "Downvoted:", voteUser)
      } else {
        bot.sendMessage(bot.Channel, "Error:", USER_NOT_FOUND_ERROR)
      }
    }
  }
  return nil
}

//Prints a help menu to the channel
func (bot *IRCBot) help() {
  bot.sendMessage(bot.Channel, "Commands are:")
  bot.sendMessage(bot.Channel, CMD_HELP)
  bot.sendMessage(bot.Channel, CMD_VOTE_UP)
  bot.sendMessage(bot.Channel, CMD_VOTE_DOWN)
  bot.sendMessage(bot.Channel, CMD_VOTES)
}

//Gets the current list of votes and sends them to the
//server
func (bot *IRCBot) getVotes() {
  bot.sendMessage(bot.Channel, "Current votes are:")
  for key, value := range bot.Votes {
    bot.sendMessage(bot.Channel, key, ":", strconv.Itoa(value))
  }
}


func (bot *IRCBot) setUsers(line string) {
  fmt.Printf("Got names, setting users\n")
  nameLine := line[strings.Index(line, bot.Channel)+len(bot.Channel)+2:len(line)]
  names := strings.Split(nameLine, " ")
  for index, curName := range names {
    if strings.HasPrefix(curName, "@") || strings.HasPrefix(curName, "+") {
      names[index] = curName[1:len(curName)]
    }
  }
  bot.Users = names
  for index, curName := range bot.Users {
    fmt.Printf("bot.Users[%d]=%s\n", index, curName)
  }
}

//checks if the user listed is in the channel
func (bot *IRCBot) inChannel(name string) (bool, error) {
  for _, curName := range bot.Users {
    if curName == name {
      return true, nil
    }
  }
  return false, nil
}

/*
//Gets a list of the names in the channel and creates a []string from them
func (bot *IRCBot) getNames() ([]string, error) {
  bot.sendCommand("NAMES", bot.Channel)
  line, err := respReq.ReadLine()
  if err != nil {
    return nil, err
  }
  nameLine := line[strings.Index(line, bot.Channel)+len(bot.Channel)+2:len(line)]
  names := strings.Split(nameLine, " ")
  for index, curName := range names {
    if strings.HasPrefix(curName, "@") {
      names[index] = curName[1:len(curName)]
    }
  }
  return names, nil
}
*/

/*
func indexOf(value string, mySlice []string) int {
  for index, curValue := range mySlice {
    if curValue == value {
      return index
    }
  }
  return -1
}
*/
