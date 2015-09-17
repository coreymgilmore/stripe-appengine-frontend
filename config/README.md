###Config File Notes:

1. All files in this directory (and subdirectories) are trimmed of whitespace and newlines.  There are no "Enters" at the end of the files.
  - Aka each file's contents should be on one line.
2. These files are read by the app when it initiates.  These files store secrets and basic settings and it is easier to store these in files than having to edit the actual code.  Once you get the app running, there is no reason to change the contents of these files.
3. All the files in this directory are ".gitignored" so you will need to create these files yourself if you develop or deploy this app. Please see details below on creating these files.

***

###Details on Each File:
#####session-auth-key.txt
- This file is used for authenticating session data and tokens.
- For details on secure session, please see http://www.gorillatoolkit.org/pkg/sessions.
- Requirements:
  - *This file is required.*
  - The contents of this file is a text string that is 64 random characters long without whitespace.
  - The contents *should not* be legible text (words, phrases, etc.).

#####session-encrypt-key.txt
- This file encrypts session data and tokens.
- For details on secure session, please see http://www.gorillatoolkit.org/pkg/sessions.
- Requirements:
  - *This file is required.*
  - The contents of this file is a text string that is exactly 32 random characters long without whitespace.
  - The contents *should not* be legible text (words, phrases, etc.)
  - Do not make this similar to the "session-auth-key.txt" file.

#####stripe-secret-key.txt
- This is your Stripe (http://www.stripe.com) secret key (test or live).
- You get this by logging into Stripe, choose "Your account", choose "Acount Settings", and select the "API Keys" tab.
- Requirements:
  - *This file is required.*
  - The file should start with "sk_".

#####statement-descriptor.txt
- This is the identifying phrase that will show up on the purchaser's credit card statement.
- Requirements:
  - *This file is required.*
  - It is a max of 22 characters long and will be trimmed as needed.
