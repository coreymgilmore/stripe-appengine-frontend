Notes
*****
- All these files are saved in this directory without whitespace padding on the beginning or end (no newlines).
- These files are read by the app when the app initiates.
- Most of the files in this directory are "gitignored" so secrets are not shared.
- Storing data in this manner keeps your secrets in easily accessible files instead of having to read through and edit code.
- These files DO NOT EXIST. You must CREATE them.


session-auth-key.txt
********************
- **this is required**
- used for authenticating session tokens created using github.com/gorilla/session.
- this key *must* be 64 bytes long (64 random characters, no white space).

session-encrypt-key.txt
***********************
- **this is required**
- used for encrypting session tokens using github.com/gorilla/session.
- this key *must* be 32 bytes long (32 random characters, no white space).

stripe-secret-key.txt
**********************
- **this is required**
- your Stripe.com secret key (test or live) for charging credit cards.
- You get this by logging into Stripe.com, choose "Your account", choose "Acount Settings", and select the "API Keys" tab.

statement-descriptor.txt
************************
- this is the phase that will show up on the purchaser's credit card statment.
- it is a max of 22 characters long.
- 'inv: + <inv_num>' is automatically append to the end of the statement descriptor.


receipt/ (directory)
********************
- **all files are required**
- These files hold your company's data (the company processing the credit cards).
- This data shows up on the credit card receipts that can be printed/saved.

receipt/city.txt
*****************
- The city in which you are located.

receipt/company-name.txt
*************************
- Your company's name as would be recognized by your customers.

receipt/country.txt
*******************
- Two or three digit country code.
- i.e.: USA or US

receipt/phone-num.txt
**********************
- You phone number.
- Can be styled in any manner you want, such as:
	- (555)-555-5555
	- 555.555.5555
	- 555-555-5555
	- +1-555-555-5555

receipt/postal-code.txt
************************
- Your company's postal code or zip code.

receipt/state.txt
******************
- The state or province in which your company is located (initials).
- i.e.: AL, NY, TX

receipt/street.txt
******************
- Your company's full street address.