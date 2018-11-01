# An App for Collecting and Charging Credit Cards

#### Intro:

#### Changelog:
Version 4 (November 2018) has breaking changes.  See `changelog.txt`.

#### What can you do with this app?:
1. Add credit cards to charge now or in the future.
2. Remove cards that already exist.
3. Charge credit cards and refund charges.
4. View transaction reports (list of charges and refunds).
5. Add or remove users as needed.
6. Control users' permissions to add, remove, charge cards, and view reports.
8. Set your own Statement Descriptor so your customers recognize your charge.
9. Print receipts.
10. Make API-style requests to autofill the card, amount, invoice, and purchase order.

#### Who should use this app?:
- Any company who processes credit cards via a virtual terminal.
- Any company who processes non-ecommerce style orders.
- Any company who saves customers' cards and then processes the card when the customer places an order.
- Best used by companies who still receive orders via phone, email, or fax or where a card may be a corporate card and the purchasing person does not know of it.

#### How it works:
1. You create a new customer by providing the customer's name and card information.
2. The card data is saved to Stripe and an ID is saved to the app.
3. When you want to charge a card the ID is set to Stripe.
4. Stripe looks up the credit card's information and processes the charge.
5. If the charge is successful, a receipt is shown.  If the card was declined, an error is shown.
6. Print a receipt or view a daily transaction log.

#### Limitations:
- Currency is currently hardcoded as USD (as is the $ symbol).
- You can only store one credit card per customer.

#### Install & Setup:
- Deployment methods:
    - Google App Engine (tested on Gen. 2, Standard Environment).
    - Any system with Golang (using SQLite as the database and your choice of web host/proxy).

- Please see `INSTALL.md` for more thorough instructions.
- As of version 4 (November 2018), this app can be run on Google App Engine or deployed locally.

1. Install Go.
2. Create a Google Cloud project.
3. Install the Google Cloud SDK.
4. Create a Stripe account.
5. Download this app.
6. Configure.
7. Deploy.
8. Run it!

####This app uses the following:
- [Boostrap](http://getbootstrap.com/) - Basic layout and html elements.
- [jQuery](https://jquery.com/) - Javascript library.
- [Stripe](https://stripe.com/) - Payment processing.
- [Google App Engine](https://cloud.google.com/appengine/docs) - Hosting platform.
- [Golang](https://golang.org/) - Backend programming language, web server.
- [Alice](https://github.com/justinas/alice) - Go middleware handler.
- [Gorilla Mux](https://github.com/gorilla/mux) - Go http router.
- [Gorilla Sessions](https://github.com/gorilla/sessions) - Secure sessions.

***

#### Contributing, Issues, New Feature
Create an issue or a pull request!