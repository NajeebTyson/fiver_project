package main

import (
	b64 "encoding/base64"
	"encoding/gob"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gorilla/sessions"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

// Session variables
var KEY = []byte("super-secret-key")
var STORE = sessions.NewCookieStore(KEY)
var USER_SESSION = "user-session"
var ADMIN_SESSION = "admin-session"
var AUTHENTICATED = "authenticated"
var PERSON_SESSION_NAME = "person"
var PERSON_TYPE = "person-type"
var USER_ADMIN = "admin"
var USER_PERSON = "user"

// Global variables
var PORT int = 3000

// var DB_NAME string = "fiverProject"
var DB_NAME string = "chatbot_data"

// var DB_URL string = "mongodb://127.0.0.1:27017/"
var DB_URL string = "mongodb://pkbotuser:impkbotnewuser77@ds111535.mlab.com:11535/chatbot_data"
var DB_COLLECTION_PERSON string = "person"
var DB_COLLECTION_ADMIN_PERSON string = "adminPerson"

var dbConnection *mgo.Session

func init() {
	gob.Register(&Person{})
}

func main() {
	// database session
	var err error
	dbConnection, err = mgo.Dial(DB_URL)
	if err != nil {
		log.Print("Database connection error")
		log.Print(err)
	}
	defer dbConnection.Close()

	// page handling
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.HandleFunc("/login", loginPageHandler)
	http.HandleFunc("/logout", logoutPageHandler)
	http.HandleFunc("/admin-logout", adminLogoutPageHandler)
	http.HandleFunc("/admin-login", adminLoginPageHandler)
	http.HandleFunc("/registration", registrationPageHandler)
	http.HandleFunc("/admin-registration", adminRegistrationPageHandler)
	http.HandleFunc("/admin-dashboard", adminDashboardPageHandler)
	http.HandleFunc("/user-dashboard", userDashboardPageHandler)
	http.HandleFunc("/view-new-members", viewNewMembersViewHandler)
	http.HandleFunc("/edit-new-members", viewNewMembersEditHandler)
	http.HandleFunc("/remove-new-members", viewNewMembersDeleteHandler)
	http.HandleFunc("/kyc-approved-members", viewKycApprovedHandler)
	http.HandleFunc("/kyc-pending-members", viewKycPendingHandler)
	http.HandleFunc("/all-members", viewAllMembersHandler)

	http.HandleFunc("/view-user", userViewHandler)
	http.HandleFunc("/view-user-final", userStaticViewHandler)
	http.HandleFunc("/edit-user", userEditHandler)
	http.HandleFunc("/remove-user", userRemoveHandler)
	http.HandleFunc("/", landingPageHandler)

	log.Printf("Server is listening at port: %d.\n", PORT)
	http.ListenAndServe(":"+strconv.Itoa(PORT), nil)
}

func landingPageHandler(res http.ResponseWriter, req *http.Request) {
	if req.Method == "GET" {
		session, _ := STORE.Get(req, ADMIN_SESSION)
		auth, ok := session.Values[AUTHENTICATED].(bool)
		if ok && auth {
			user_auth, user_ok := session.Values[PERSON_TYPE].(string)
			if user_ok && user_auth == USER_ADMIN {
				http.Redirect(res, req, "/admin-dashboard", http.StatusSeeOther)
				return
			} else if user_ok && user_auth == USER_PERSON {
				http.Redirect(res, req, "/user-dashboard", http.StatusSeeOther)
				return
			}
		}
		// fmt.Println("Not author")
		landingPageTemplate, err := template.ParseFiles("./view/index.html")
		if err != nil {
			log.Fatal("Error in parsing template file: Home")
			log.Fatal(err)
		}
		landingPageTemplate.Execute(res, nil)
	} else {
		res.WriteHeader(404)
	}
}

func logoutPageHandler(res http.ResponseWriter, req *http.Request) {
	if req.Method == "GET" {
		session, _ := STORE.Get(req, USER_SESSION)
		session.Values[AUTHENTICATED] = false
		session.Values[PERSON_TYPE] = ""
		session.Save(req, res)
		http.Redirect(res, req, "/", http.StatusSeeOther)
	}
}

func adminLogoutPageHandler(res http.ResponseWriter, req *http.Request) {
	if req.Method == "GET" {
		session, _ := STORE.Get(req, ADMIN_SESSION)
		session.Values[AUTHENTICATED] = false
		session.Values[PERSON_TYPE] = ""
		session.Save(req, res)
		http.Redirect(res, req, "/", http.StatusSeeOther)
	}
}

func loginPageHandler(res http.ResponseWriter, req *http.Request) {
	if req.Method == "GET" {
		loginPageTemplate, err := template.ParseFiles("./view/login.html")
		if err != nil {
			fmt.Fprintf(res, "Error in parsing template file")
			log.Fatal(err)
			return
		}
		session, _ := STORE.Get(req, USER_SESSION)
		var message interface{}
		if flashes := session.Flashes(); len(flashes) > 0 {
			message = flashes
		}
		session.Save(req, res)
		loginPageTemplate.Execute(res, map[string]interface{}{"message": message})
	} else if req.Method == "POST" {

		if err := req.ParseForm(); err != nil {
			fmt.Fprintf(res, "ParseForm() err: %v", err)
			return
		}
		session, _ := STORE.Get(req, USER_SESSION)

		person := Person{Username: req.FormValue("username"), Password: req.FormValue("password")}

		var foundPerson Person
		personCollection := dbConnection.DB(DB_NAME).C(DB_COLLECTION_PERSON)
		personCollection.Find(bson.M{"username": person.Username, "password": person.Password}).One(&foundPerson)

		if foundPerson.Username == person.Username {
			if foundPerson.Address2 == "" {
				foundPerson.Address2 = "nil"
			}
			foundPerson.Document = []byte("")
			session.Values[AUTHENTICATED] = true
			session.Values[PERSON_SESSION_NAME] = foundPerson
			session.Values[PERSON_TYPE] = USER_PERSON
			session.Save(req, res)
			http.Redirect(res, req, "/user-dashboard", http.StatusSeeOther)
		} else {
			session.AddFlash("Invalid email or password.")
			session.Save(req, res)
			http.Redirect(res, req, "/login", http.StatusSeeOther)
		}
	} else {
		res.WriteHeader(404)
	}
}

func registrationPageHandler(res http.ResponseWriter, req *http.Request) {
	if req.Method == "GET" {
		loginPageTemplate, err := template.ParseFiles("./view/registration.html")
		if err != nil {
			fmt.Fprintf(res, "Error in parsing template file")
			log.Fatal(err)
			return
		}
		loginPageTemplate.Execute(res, nil)
	} else if req.Method == "POST" {
		if err := req.ParseForm(); err != nil {
			fmt.Fprintf(res, "ParseForm() err: %v", err)
			return
		}
		// file handling
		file, header, err := req.FormFile("document")
		if err != nil {
			panic(err)
		}
		defer file.Close()
		fileBytes, err := ioutil.ReadAll(file)

		// inserting person data
		person := Person{
			Name:         req.FormValue("name"),
			Gender:       req.FormValue("gender"),
			Dob:          req.FormValue("dob"),
			Nationality:  req.FormValue("nationality"),
			Address1:     req.FormValue("address1"),
			Address2:     req.FormValue("address2"),
			Country:      req.FormValue("country"),
			Email:        req.FormValue("email"),
			Username:     req.FormValue("username"),
			Password:     req.FormValue("password"),
			Passport:     req.FormValue("passport"),
			Mobile:       req.FormValue("mobile"),
			Documentname: header.Filename,
			Document:     fileBytes,
			Kycstatus:    "pending",
			Aml:          "pending",
			Cft:          "pending",
			Bankname:     "",
			Chequeno:     "",
			Amount:       "0",
			Memberstatus: "new"}

		personCollection := dbConnection.DB(DB_NAME).C(DB_COLLECTION_PERSON)
		e := personCollection.Insert(&person)
		if e != nil {
			log.Fatal("Error: ", e)
		}
		http.Redirect(res, req, "/login", http.StatusSeeOther)
	} else {
		res.WriteHeader(404)
	}
}

func adminLoginPageHandler(res http.ResponseWriter, req *http.Request) {
	if req.Method == "GET" {
		loginPageTemplate, err := template.ParseFiles("./view/admin_login.html")
		if err != nil {
			fmt.Fprintf(res, "Error in parsing template file")
			log.Fatal(err)
			return
		}
		session, _ := STORE.Get(req, ADMIN_SESSION)
		var message interface{}
		if flashes := session.Flashes(); len(flashes) > 0 {
			message = flashes
		}
		session.Save(req, res)
		loginPageTemplate.Execute(res, map[string]interface{}{"message": message})
	} else if req.Method == "POST" {
		if err := req.ParseForm(); err != nil {
			fmt.Fprintf(res, "ParseForm() err: %v", err)
			return
		}
		session, _ := STORE.Get(req, ADMIN_SESSION)
		person := AdminPerson{
			Username: req.FormValue("username"),
			Password: req.FormValue("password")}
		var foundPerson AdminPerson
		personCollection := dbConnection.DB(DB_NAME).C(DB_COLLECTION_ADMIN_PERSON)
		personCollection.Find(bson.M{"username": person.Username, "password": person.Password}).One(&foundPerson)
		if person.Username == foundPerson.Username {
			session.Values[AUTHENTICATED] = true
			session.Values[PERSON_TYPE] = USER_ADMIN
			session.Values["name"] = foundPerson.Name
			// session.Values[PERSON_SESSION_NAME] = foundPerson
			session.Save(req, res)
			http.Redirect(res, req, "/admin-dashboard", http.StatusSeeOther)
		} else {
			session.AddFlash("Invalid email or password.")
			session.Save(req, res)
			http.Redirect(res, req, "/admin-login", http.StatusSeeOther)
		}
	} else {
		res.WriteHeader(404)
	}
}

func adminRegistrationPageHandler(res http.ResponseWriter, req *http.Request) {
	if req.Method == "GET" {
		loginPageTemplate, err := template.ParseFiles("./view/admin_registration.html")
		if err != nil {
			fmt.Fprintf(res, "Error in parsing template file")
			log.Fatal(err)
			return
		}
		loginPageTemplate.Execute(res, nil)
	} else if req.Method == "POST" {
		if err := req.ParseForm(); err != nil {
			fmt.Fprintf(res, "ParseForm() err: %v", err)
			return
		}
		// inserting person data
		adminPerson := AdminPerson{
			Name:     req.FormValue("name"),
			Username: req.FormValue("username"),
			Password: req.FormValue("password")}

		personCollection := dbConnection.DB(DB_NAME).C(DB_COLLECTION_ADMIN_PERSON)
		e := personCollection.Insert(&adminPerson)
		if e != nil {
			log.Fatal("Error: ", e)
		}
		http.Redirect(res, req, "/admin-dashboard", http.StatusSeeOther)
	} else {
		res.WriteHeader(404)
	}
}

func adminDashboardPageHandler(res http.ResponseWriter, req *http.Request) {
	if req.Method == "GET" {
		dashboardPageTemplate, err := template.ParseFiles("./view/admin_dashboard.html")
		if err != nil {
			fmt.Fprintf(res, "Error in parsing template file")
			log.Fatal(err)
			return
		}
		session, _ := STORE.Get(req, ADMIN_SESSION)
		auth, ok := session.Values[AUTHENTICATED].(bool)
		if ok && auth {
			admin_auth, admin_ok := session.Values[PERSON_TYPE].(string)
			if admin_ok && admin_auth == USER_ADMIN {
				val, _ := session.Values["name"].(string)
				var person = &AdminPerson{}
				person.Name = val
				dashboardPageTemplate.Execute(res, &person)
				return
			} else {
				session.Values[AUTHENTICATED] = false
				session.Values[PERSON_TYPE] = ""
				session.Save(req, res)
			}
		}
		http.Redirect(res, req, "/admin-login", http.StatusSeeOther)
	}
}

func userDashboardPageHandler(res http.ResponseWriter, req *http.Request) {
	if req.Method == "GET" {
		dashboardPageTemplate, err := template.ParseFiles("./view/dashboard.html")
		if err != nil {
			fmt.Fprintf(res, "Error in parsing template file")
			log.Fatal(err)
			return
		}
		session, _ := STORE.Get(req, USER_SESSION)
		// Check if user is authenticated
		auth, ok := session.Values[AUTHENTICATED].(bool)
		if ok && auth {
			user_auth, user_ok := session.Values[PERSON_TYPE].(string)
			if user_ok && user_auth == USER_PERSON {
				val := session.Values[PERSON_SESSION_NAME]
				var person = &Person{}
				person, ok := val.(*Person)
				if ok {
					dashboardPageTemplate.Execute(res, &person)
					return
				}
			} else {
				session.Values[AUTHENTICATED] = false
				session.Values[PERSON_TYPE] = ""
				session.Save(req, res)
			}
		}
		http.Redirect(res, req, "/login", http.StatusSeeOther)
	}
}

func viewNewMembersViewHandler(res http.ResponseWriter, req *http.Request) {
	if req.Method == "GET" {
		session, _ := STORE.Get(req, ADMIN_SESSION)
		auth, ok := session.Values[AUTHENTICATED].(bool)
		admin_auth, admin_ok := session.Values[PERSON_TYPE].(string)
		if (ok && auth) && (admin_ok && admin_auth == USER_ADMIN) {
			var newPersons []Person
			personCollection := dbConnection.DB(DB_NAME).C(DB_COLLECTION_PERSON)
			personCollection.Find(bson.M{"memberstatus": "new"}).Select(bson.M{"username": 1, "name": 1, "email": 1, "passport": 1, "mobile": 1, "dob": 1, "memberstatus": 1}).All(&newPersons)

			newMemberViewPageTemplate, err := template.ParseFiles("./view/new_members.html")
			if err != nil {
				fmt.Fprintf(res, "Error in parsing template file")
				log.Fatal(err)
				return
			}
			newMemberViewPageTemplate.Execute(res, map[string]interface{}{"Title": "New Members",
				"Persons":  newPersons,
				"Link":     "/view-user?u=",
				"LinkName": "view"})
		} else {
			http.Redirect(res, req, "/admin-login", http.StatusSeeOther)
		}
	} else {
		res.WriteHeader(404)
	}
}

func viewNewMembersEditHandler(res http.ResponseWriter, req *http.Request) {
	if req.Method == "GET" {
		session, _ := STORE.Get(req, ADMIN_SESSION)
		auth, ok := session.Values[AUTHENTICATED].(bool)
		admin_auth, admin_ok := session.Values[PERSON_TYPE].(string)
		if (ok && auth) && (admin_ok && admin_auth == USER_ADMIN) {
			var newPersons []Person
			personCollection := dbConnection.DB(DB_NAME).C(DB_COLLECTION_PERSON)
			personCollection.Find(bson.M{"memberstatus": "new"}).Select(bson.M{"username": 1, "name": 1, "email": 1, "passport": 1, "mobile": 1, "dob": 1, "memberstatus": 1}).All(&newPersons)

			newMemberViewPageTemplate, err := template.ParseFiles("./view/new_members.html")
			if err != nil {
				fmt.Fprintf(res, "Error in parsing template file")
				log.Fatal(err)
				return
			}
			newMemberViewPageTemplate.Execute(res, map[string]interface{}{"Title": "Edit New Members",
				"Persons":  newPersons,
				"Link":     "/edit-user?u=",
				"LinkName": "edit"})
		} else {
			http.Redirect(res, req, "/admin-login", http.StatusSeeOther)
		}
	} else {
		res.WriteHeader(404)
	}
}

func viewNewMembersDeleteHandler(res http.ResponseWriter, req *http.Request) {
	if req.Method == "GET" {
		session, _ := STORE.Get(req, ADMIN_SESSION)
		auth, ok := session.Values[AUTHENTICATED].(bool)
		admin_auth, admin_ok := session.Values[PERSON_TYPE].(string)
		if (ok && auth) && (admin_ok && admin_auth == USER_ADMIN) {
			var newPersons []Person
			personCollection := dbConnection.DB(DB_NAME).C(DB_COLLECTION_PERSON)
			personCollection.Find(bson.M{"memberstatus": "new"}).Select(bson.M{"username": 1, "name": 1, "email": 1, "passport": 1, "mobile": 1, "dob": 1, "memberstatus": 1}).All(&newPersons)

			newMemberViewPageTemplate, err := template.ParseFiles("./view/new_members.html")
			if err != nil {
				fmt.Fprintf(res, "Error in parsing template file")
				log.Fatal(err)
				return
			}
			newMemberViewPageTemplate.Execute(res, map[string]interface{}{"Title": "Remove New Members",
				"Persons":  newPersons,
				"Link":     "/remove-user?u=",
				"LinkName": "remove"})
		} else {
			http.Redirect(res, req, "/admin-login", http.StatusSeeOther)
		}
	} else {
		res.WriteHeader(404)
	}
}

func viewKycApprovedHandler(res http.ResponseWriter, req *http.Request) {
	if req.Method == "GET" {
		session, _ := STORE.Get(req, ADMIN_SESSION)
		auth, ok := session.Values[AUTHENTICATED].(bool)
		admin_auth, admin_ok := session.Values[PERSON_TYPE].(string)
		if (ok && auth) && (admin_ok && admin_auth == USER_ADMIN) {
			var newPersons []Person
			personCollection := dbConnection.DB(DB_NAME).C(DB_COLLECTION_PERSON)
			personCollection.Find(bson.M{"kycstatus": "approved"}).Select(bson.M{"username": 1, "name": 1, "email": 1, "passport": 1, "mobile": 1, "dob": 1, "memberstatus": 1}).All(&newPersons)

			newMemberViewPageTemplate, err := template.ParseFiles("./view/new_members.html")
			if err != nil {
				fmt.Fprintf(res, "Error in parsing template file")
				log.Fatal(err)
				return
			}
			newMemberViewPageTemplate.Execute(res, map[string]interface{}{"Title": "KYC Approved Members",
				"Persons":  newPersons,
				"Link":     "/view-user-final?u=",
				"LinkName": "view"})
		} else {
			http.Redirect(res, req, "/admin-login", http.StatusSeeOther)
		}
	} else {
		res.WriteHeader(404)
	}
}

func viewKycPendingHandler(res http.ResponseWriter, req *http.Request) {
	if req.Method == "GET" {
		session, _ := STORE.Get(req, ADMIN_SESSION)
		auth, ok := session.Values[AUTHENTICATED].(bool)
		admin_auth, admin_ok := session.Values[PERSON_TYPE].(string)
		if (ok && auth) && (admin_ok && admin_auth == USER_ADMIN) {
			var newPersons []Person
			personCollection := dbConnection.DB(DB_NAME).C(DB_COLLECTION_PERSON)
			personCollection.Find(bson.M{"kycstatus": "pending"}).Select(bson.M{"username": 1, "name": 1, "email": 1, "passport": 1, "mobile": 1, "dob": 1, "memberstatus": 1}).All(&newPersons)
			newMemberViewPageTemplate, err := template.ParseFiles("./view/new_members.html")
			if err != nil {
				fmt.Fprintf(res, "Error in parsing template file")
				log.Fatal(err)
				return
			}
			newMemberViewPageTemplate.Execute(res, map[string]interface{}{"Title": "KYC Pending Members",
				"Persons":  newPersons,
				"Link":     "/view-user?u=",
				"LinkName": "view"})
		} else {
			http.Redirect(res, req, "/admin-login", http.StatusSeeOther)
		}
	} else {
		res.WriteHeader(404)
	}
}

func viewAllMembersHandler(res http.ResponseWriter, req *http.Request) {
	if req.Method == "GET" {
		session, _ := STORE.Get(req, ADMIN_SESSION)
		auth, ok := session.Values[AUTHENTICATED].(bool)
		admin_auth, admin_ok := session.Values[PERSON_TYPE].(string)
		if (ok && auth) && (admin_ok && admin_auth == USER_ADMIN) {
			var newPersons []Person
			personCollection := dbConnection.DB(DB_NAME).C(DB_COLLECTION_PERSON)
			personCollection.Find(bson.M{}).Select(bson.M{"username": 1, "name": 1, "email": 1, "passport": 1, "mobile": 1, "dob": 1, "memberstatus": 1}).All(&newPersons)

			newMemberViewPageTemplate, err := template.ParseFiles("./view/new_members.html")
			if err != nil {
				fmt.Fprintf(res, "Error in parsing template file")
				log.Fatal(err)
				return
			}
			newMemberViewPageTemplate.Execute(res, map[string]interface{}{"Title": "All Members",
				"Persons":  newPersons,
				"Link":     "/view-user?u=",
				"LinkName": "view"})
		} else {
			http.Redirect(res, req, "/admin-login", http.StatusSeeOther)
		}
	} else {
		res.WriteHeader(404)
	}
}

func userViewHandler(res http.ResponseWriter, req *http.Request) {
	session, _ := STORE.Get(req, ADMIN_SESSION)
	auth, ok := session.Values[AUTHENTICATED].(bool)
	admin_auth, admin_ok := session.Values[PERSON_TYPE].(string)
	if (ok && auth) && (admin_ok && admin_auth == USER_ADMIN) {
		if req.Method == "GET" {
			userName := req.URL.Query().Get("u")

			var person Person
			personCollection := dbConnection.DB(DB_NAME).C(DB_COLLECTION_PERSON)
			personCollection.Find(bson.M{"username": userName}).One(&person)
			if person.Address2 == "" {
				person.Address2 = "Nil"
			}
			adminUserViewPageTemplate, err := template.ParseFiles("./view/admin_view.html")
			if err != nil {
				fmt.Fprintf(res, "Error in parsing template file")
				log.Fatal(err)
				return
			}
			imageEnc := b64.StdEncoding.EncodeToString(person.Document)
			person.Document = []byte("")
			adminUserViewPageTemplate.Execute(res, map[string]interface{}{"person": person, "image": imageEnc})
		} else if req.Method == "POST" {
			userName := req.URL.Query().Get("u")
			if err := req.ParseForm(); err != nil {
				fmt.Fprintf(res, "ParseForm() err: %v", err)
				return
			}
			kycStatus := req.FormValue("kyc")
			amlStatus := req.FormValue("aml")
			cftStatus := req.FormValue("cft")
			bankName := req.FormValue("bankname")
			chequeNo := req.FormValue("chequeno")
			amount := req.FormValue("amount")

			personCollection := dbConnection.DB(DB_NAME).C(DB_COLLECTION_PERSON)
			personCollection.Update(bson.M{"username": userName}, bson.M{"$set": bson.M{
				"memberstatus": "processed",
				"kycstatus":    kycStatus,
				"aml":          amlStatus,
				"cft":          cftStatus,
				"chequeno":     chequeNo,
				"bankname":     bankName,
				"amount":       amount}})
			http.Redirect(res, req, "/admin-dashboard", http.StatusSeeOther)
		} else {
			res.WriteHeader(404)
		}
	} else {
		http.Redirect(res, req, "/admin-login", http.StatusSeeOther)
	}
}

func userStaticViewHandler(res http.ResponseWriter, req *http.Request) {
	session, _ := STORE.Get(req, ADMIN_SESSION)
	auth, ok := session.Values[AUTHENTICATED].(bool)
	admin_auth, admin_ok := session.Values[PERSON_TYPE].(string)
	if (ok && auth) && (admin_ok && admin_auth == USER_ADMIN) {
		if req.Method == "GET" {
			userName := req.URL.Query().Get("u")

			var person Person
			personCollection := dbConnection.DB(DB_NAME).C(DB_COLLECTION_PERSON)
			personCollection.Find(bson.M{"username": userName}).One(&person)
			if person.Address2 == "" {
				person.Address2 = "Nil"
			}
			adminUserViewPageTemplate, err := template.ParseFiles("./view/admin_static_view.html")
			if err != nil {
				fmt.Fprintf(res, "Error in parsing template file")
				log.Fatal(err)
				return
			}
			imageEnc := b64.StdEncoding.EncodeToString(person.Document)
			person.Document = []byte("")
			adminUserViewPageTemplate.Execute(res, map[string]interface{}{"person": person, "image": imageEnc})
		} else {
			res.WriteHeader(404)
		}
	} else {
		http.Redirect(res, req, "/admin-login", http.StatusSeeOther)
	}
}

func userEditHandler(res http.ResponseWriter, req *http.Request) {
	session, _ := STORE.Get(req, ADMIN_SESSION)
	auth, ok := session.Values[AUTHENTICATED].(bool)
	admin_auth, admin_ok := session.Values[PERSON_TYPE].(string)
	if (ok && auth) && (admin_ok && admin_auth == USER_ADMIN) {
		if req.Method == "GET" {
			userName := req.URL.Query().Get("u")

			var person Person
			personCollection := dbConnection.DB(DB_NAME).C(DB_COLLECTION_PERSON)
			personCollection.Find(bson.M{"username": userName}).One(&person)
			if person.Address2 == "" {
				person.Address2 = "Nil"
			}
			adminUserViewPageTemplate, err := template.ParseFiles("./view/edit_user.html")
			if err != nil {
				fmt.Fprintf(res, "Error in parsing template file")
				log.Fatal(err)
				return
			}
			adminUserViewPageTemplate.Execute(res, person)
		} else if req.Method == "POST" {
			if err := req.ParseForm(); err != nil {
				fmt.Fprintf(res, "ParseForm() err: %v", err)
				return
			}

			userName := req.URL.Query().Get("u")
			person := Person{
				Name:        req.FormValue("name"),
				Gender:      req.FormValue("gender"),
				Dob:         req.FormValue("dob"),
				Nationality: req.FormValue("nationality"),
				Address1:    req.FormValue("address1"),
				Address2:    req.FormValue("address2"),
				Country:     req.FormValue("country"),
				Passport:    req.FormValue("passport"),
				Mobile:      req.FormValue("mobile")}
			personCollection := dbConnection.DB(DB_NAME).C(DB_COLLECTION_PERSON)
			personCollection.Update(bson.M{"username": userName}, bson.M{"$set": bson.M{
				"name":        person.Name,
				"gender":      person.Gender,
				"dob":         person.Dob,
				"nationality": person.Nationality,
				"address1":    person.Address1,
				"address2":    person.Address2,
				"country":     person.Country,
				"passport":    person.Passport,
				"mobile":      person.Mobile}})

			http.Redirect(res, req, "/admin-dashboard", http.StatusNotModified)
		} else {
			res.WriteHeader(404)
		}
	} else {
		http.Redirect(res, req, "/admin-login", http.StatusSeeOther)
	}
}

func userRemoveHandler(res http.ResponseWriter, req *http.Request) {
	session, _ := STORE.Get(req, ADMIN_SESSION)
	auth, ok := session.Values[AUTHENTICATED].(bool)
	admin_auth, admin_ok := session.Values[PERSON_TYPE].(string)
	if (ok && auth) && (admin_ok && admin_auth == USER_ADMIN) {
		if req.Method == "POST" {
			if err := req.ParseForm(); err != nil {
				fmt.Fprintf(res, "ParseForm() err: %v", err)
				return
			}
			userName := req.FormValue("username")
			personCollection := dbConnection.DB(DB_NAME).C(DB_COLLECTION_PERSON)
			err := personCollection.Remove(bson.M{"username": userName})
			if err != nil {
				res.Write([]byte("not_done"))
			} else {
				res.Write([]byte("done"))
			}
		} else {
			res.WriteHeader(404)
		}
	} else {
		http.Redirect(res, req, "/admin-login", http.StatusSeeOther)
	}
}

func getDBConnection() *mgo.Database {
	dbConnection, err := mgo.Dial(DB_URL)
	if err != nil {
		log.Print("Database connection error")
		log.Print(err)
	}
	db := dbConnection.DB(DB_NAME)
	return db
}

func saveDocument(filename string, document []byte) {
	newFile, err := os.Create(filename)
	if err != nil {
		log.Fatal("CANT_WRITE_FILE", http.StatusInternalServerError)
		return
	}
	defer newFile.Close()
	if _, err := newFile.Write(document); err != nil {
		log.Fatal("CANT_WRITE_FILE", http.StatusInternalServerError)
		return
	}
}

// Database models
type AdminPerson struct {
	Name     string `bson:"name" json:"name"`
	Username string `bson:"username" json:"username"`
	Password string `bson:"password" json:"password"`
}

type Person struct {
	// ID   bson.ObjectId
	Name         string `bson:"name" json:"name"`
	Gender       string `bson:"gender" json:"gender"`
	Nationality  string `bson:"nationality" json:"nationality"`
	Address1     string `bson:"address1" json:"address1"`
	Address2     string `bson:"address2" json:"address2"`
	Country      string `bson:"country" json:"country"`
	Email        string `bson:"email" json:"email"`
	Username     string `bson:"username" json:"username"`
	Password     string `bson:"password" json:"password"`
	Dob          string `bson:"dob" json:"dob"`
	Kycstatus    string `bson:"kycstatus" json:"kycstatus"`
	Memberstatus string `bson:"memberstatus" json:"memberstatus"`
	Passport     string `bson:"passport" json:"passport"`
	Documentname string `bson:"documentname" json:"documentname"`
	Document     []byte `bson:"document" json:"document"`
	Mobile       string `bson:"mobile" json:"mobile"`
	Aml          string `bson:"aml" json:"aml"`
	Cft          string `bson:"cft" json:"cft"`
	Chequeno     string `bson:"chequeno" json:"chequeno"`
	Bankname     string `bson:"bankname" json:"bankname"`
	Amount       string `bson:"amount" json:"amount"`
}
