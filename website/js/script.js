//*******************************************************************************
//CONSTANTS
const MIN_PASSWORD_LENGTH = 8;
const BAD_PASSWORDS = [
	"password",
	"password1",
	"12345678",
	"123456789",
	"123123123",
	"00000000",
	"1234567890",
	"asdfasdf",
	"asdfghjkl",
	"testtest",
	"admin@example.com"
];

//*******************************************************************************
//STRIPE PUBLIC KEY
Stripe.setPublishableKey('pk_test_pKzD1QYPWJNrwuGTZ2k0HEkn');

//*******************************************************************************
//COMMON FUNCS

//ESCAPE STRINGS
//remove HTML characters/tags from strings used for saving data to db
//http://stackoverflow.com/questions/6234773/can-i-escape-html-special-chars-in-javascript
function escapeHTML(string) {
	return string
		.replace(/&/g, "&amp;")
		.replace(/</g, "&lt;")
		.replace(/>/g, "&gt;")
		.replace(/"/g, "&quot;")
		.replace(/'/g, "&#039;");
}

//REGEX MATCH EMAIL
//true if input is a valid email
function validateEmail(email) {
	var regex = /^(([^<>()[\]\\.,;:\s@\"]+(\.[^<>()[\]\\.,;:\s@\"]+)*)|(\".+\"))@((\[[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\])|(([a-zA-Z\-0-9]+\.)+[a-zA-Z]{2,}))$/;
	return regex.test(email);
}

//CHECK IF TWO PASSWORDS MATCH
function doWordsMatch (word1, word2) {
	if (word1 === word2) {
		return true;
	}

	return false;
}

//CHECK IF A PASSWORD IS LONG ENOUGH
function isLongPassword (password) {
	if (password.length < MIN_PASSWORD_LENGTH) {
		return false;
	}

	return true;
}

//CHECK IF A GIVEN PASSWORD IS IN THE LIST OF TOO EASY TO GUESS PASSWORDS
function isSimplePassword (password) {
	if (BAD_PASSWORDS.indexOf(password) !== -1) {
		return true;
	}

	return false;
}

//SHOW AN ERROR MESSAGE IN A FORM
//title is usually "error" or "warning"
//message is something descriptive about the error so the user can fix the problem
//parentElem is the parent html element where the <div #error-msg> is located
function showFormErrorMsg (title, message, parentElem) {
	var alert = "" + 
		"<div class='alert alert-danger'>" + 
			"<b>" + title + "</b> " + message + 
		"</div>";
	parentElem.find('#error-msg').html(alert);
	return;
}

//SHOW MESSAGES IN MODALS
function showModalMessage (msg, type, elem) {
	elem.html('<div class="alert alert-' + type + '">' + msg + '</div>');
	return
}

//*******************************************************************************
//FUNCS

//SHOW SPECIFIC ACTION PANEL WHEN NAV BUTTONS ARE CLICKED
$('body').on('click', '.action-btn', function() {
	//get data attribute which tells us which panel to show
	var btn = 			$(this);
	var dataAction = 	btn.data("action");

	//make sure buttons arent disabled
	if (btn.attr("disabled") !== undefined) {
		console.log("btn disabled");
		return;
	}

	//make sure the button clicked isn't already showing the correct panel
	var showingPanel = 		$('.action-panels.show');
	var showingPanelId = 	showingPanel.attr('id');
	if (showingPanelId === dataAction) {
		return;
	}

	//generate selector of new panel to show
	var newPanel = 			"#" + dataAction;
	var newPanelElem = 		$(newPanel);

	//get width of action-panel container to define slide in/out distance
	//so the panel slides completely out of view before new panel slides in
	//width changes bases on browser window size
	var containerWidth = 	$('#action-panels-container').outerWidth()

	//slide the current panel out to the left
	//slide the new panel in to the right
	//disabled all the buttons when sliding is occuring
	var allBtns = $('.action-btn');
	allBtns.attr("disabled", true).children('input').attr("disabled", true);
	showingPanel.toggle('slide', {distance: containerWidth}, 600, function() {
		showingPanel.removeClass('show');

		newPanelElem.toggle('slide', 600, function() {
			newPanelElem.addClass('show');
			allBtns.attr("disabled", false);
		});
	});
});

//VALIDATE THE INITIAL ADMIN CREATE FORM
//make sure the passwords are strong and match
$('#create-init-admin').submit(function (e) {
	//get passwords from inputs
	var pass1 = $('#password1').val();
	var pass2 = $('#password2').val();

	//check if passwords match
	if (doWordsMatch(pass1, pass2) === false) {
		e.preventDefault();
		showFormErrorMsg("Error", "The passwords do not match.", $('#create-init-admin'));
		return false;
	}

	//make sure the password is long enough
	if (isLongPassword(pass1) === false) {
		e.preventDefault();
		showFormErrorMsg("Error", "Your password is too short. It must be at least " + MIN_PASSWORD_LENGTH + " characters.", $('#create-init-admin'));
		return false;
	}

	//make sure the passwords are not very easy to guess
	if (isSimplePassword(pass1) === true) {
		e.preventDefault();
		showFormErrorMsg("Error", "The password you provided is too simple. Please choose a better password.", $('#create-init-admin'));
		return false;
	}

	//passwords validated
	//form will submit and save admin user
	//user will see a success/error page
	//then user can log in on main login page
});

//GENERATE LIST OF YEARS FOR CARD EXPIRATION
//done on page load
//fills in a <select> with <options>
$(function() {
	var elem = $('#card-exp-year');
	elem.html("");

	//get current year
	var d = 	new Date()
	var year = 	d.getFullYear();

	//default first value
	elem.append("<option value='0'>Please choose.</option>");

	//options for years
	for (var i = year; i < year + 11; i ++) {
		elem.append("<option value='" + i + "'>" + i + "</option>");
	}

	console.log("Loaded list of expiration years.");
});

//HIDE "THIS YEAR" IF USER CHOOSES AN EXPIRATION MONTH IN THE PAST
//user cannot choose an expiration in a past month for this year
$('#add-card').on('change', '#card-exp-month', function() {
	//get value from month chosen
	var expMonth = 		$(this).val();

	//get current month
	var d = 			new Date();
	var currentMonth = 	d.getMonth() + 1;
	var currentYear = 	d.getFullYear();

	//check if expiration month is in the past
	//hide the option for this year if month is in the past
	if (expMonth < currentMonth) {
		$('#card-exp-year option[value=' + currentYear + ']').css({"display": "none"});
	}
	else {
		$('#card-exp-year option[value=' + currentYear + ']').css({"display": "block"});
	}

	return;
});

//ADD A NEW CREDIT CARD
//validate the card data and save the card via ajax call
$('#add-card').submit(function (e) {
	var form = 			$('#add-card');
	var customerId = 	$('#customer-id').val().trim();
	var customerName = 	$('#customer-name').val().trim();
	var cardholder = 	$('#cardholder-name').val().trim();
	var cardNum = 		$('#card-number').val().trim().replace(' ', '').replace('-', '');
	var expYear = 		parseInt($('#card-exp-year').val());
	var expMonth = 		parseInt($('#card-exp-month').val());
	var cvc = 			$('#card-cvc').val().trim();
	var postal = 		$('#card-postal-code').val().trim();
	var cardType = 		Stripe.card.cardType(cardNum);

	//disable the submit button
	//so the user cannot add the same card twice by mistake
	$('#add-card #submit').prop("disabled", true).text("Adding Card...");

	//hide any existing warnings
	$('#error-msg').html('');

	//make sure each input is valid
	//customer name
	if (customerName.length < 2) {
		e.preventDefault();
		showFormErrorMsg('Error!', 'You must provide a customer name.  This can be the same as the cardholder or the name of a company.  This is used to lookup cards when you want to create a charge.', form);
		return false;
	}

	//cardholder name
	if (cardholder.length < 2) {
		e.preventDefault();
		showFormErrorMsg('Error!', 'Please provide the name of the cardholder as it is given on the card.', form);
		return false;
	}

	//card number
	var cardNumLength = cardNum.length;
	if (cardNumLength < 15 || cardNumLength > 16) {
		e.preventDefault();
		showFormErrorMsg('Error!', 'The card number you provided is ' + cardNumLength + ' digits long, however, it must be exactly 15 or 16 digits.', form);
		return false;
	}
	if (Stripe.card.validateCardNumber(cardNum) === false) {
		e.preventDefault();
		showFormErrorMsg('Error!', 'The card number you provided is not valid.', form);
		return false;
	}

	//expiration
	var d = 		new Date();
	var nowMonth = 	d.getMonth() + 1;
	var nowYear = 	d.getFullYear();
	//month
	if (expMonth === 0 || expMonth === '0') {
		e.preventDefault();
		showFormErrorMsg('Error!', 'Please choose the card\'s expiration month.', form);
		return false;
	}
	//year
	if (expYear === 0 || expYear === '0') {
		e.preventDefault();
		showFormErrorMsg('Error!', 'Please choose the card\'s expiration year.', form);
		return false;
	}
	//both
	if (expYear === nowYear && expMonth < nowMonth) {
		e.preventDefault();
		showFormErrorMsg('Error!', 'The card\'s expiration must be in the future.', form);
		return false;
	}
	if (Stripe.card.validateExpiry(expMonth, expYear) === false) {
		e.preventDefault();
		showFormErrorMsg('Error!', 'The card\'s expiration must be in the future.', form);
		return false;
	}

	//cvc
	if (Stripe.card.validateCVC(cvc) === false) {
		e.preventDefault();
		showFormErrorMsg('Error!', 'The security code you provided is invalid.', form);
		return false;
	}
	if (cardType === "American Express" && cvc.length !== 4) {
		e.preventDefault();
		showFormErrorMsg('Error!', 'You provided an American Express card but your security code is invalid. The security code must be exactly 4 numbers long.', form);
		return false;
	}
	if (cardType !== "American Express" && cvc.length !== 3) {
		e.preventDefault();
		console.log("asd");

		showFormErrorMsg('Error!', 'You provided an ' + Stripe.card.cardType(cardNum) + ' card but your security code is invalid. The security code must be exactly 3 numbers long.', form);
		return false;
	}

	//postal code
	if (postal.length < 5 || postal.length > 6) {
		e.preventDefault();
		showFormErrorMsg('Error!', 'The postal code must be exactly 5 numeric or 6 alphanumeric characters.', form);
		return false;
	}

	//create card token
	Stripe.card.createToken({
		name: 			cardholder,
		number: 		cardNum,
		cvc: 			cvc,
		exp_month: 		expMonth,
		exp_year: 		expYear,
		address_zip: 	postal
	}, createTokenCallback)

	function createTokenCallback (status, response) {
		if (response.error) {
			showFormErrorMsg('Error!', 'The credit card could not be saved. Please contact an administrator. Message: ' + response.error.message + '.', form);
			return;
		}

		//perform ajax call
		//save data to db
		//create stripe customer using card token
		$.ajax({
			type: 	"POST",
			url: 	"/card/add/",
			data: {
				customerId: 	customerId,
				customerName: 	customerName,
				cardholder: 	cardholder,
				cardToken: 		response['id'],
				cardExp: 		response['card']['exp_month'] + "/" + response['card']['exp_year'],
				cardLast4: 		response['card']['last4']
			},
			error: function (r) {
				console.log("AJAX save card error");
				console.log(r);
			},
			success: function (r) {
				console.log(r);
			}
		});

		//done
		//show user success panel
		//clear out inputs from add-card form
		//re-enable button to save new cards
		$('#customer-id').val('');
		$('#customer-name').val('');
		$('#cardholder-name').val('');
		$('#card-number').val('');
		$('#card-exp-year').val('0');
		$('#card-exp-month').val('0');
		$('#card-cvc').val('');
		$('#card-postal-code').val('');
		$('#add-card #submit').prop("disabled", false).text("Add Card");
		return;
	}


	//stop form from submitting since it won't do anything anyway
	e.preventDefault();
	return false;
});

//*******************************************************************************
//ADD USER

//ADD A NEW USER
//validate user data and save user via ajax call
$('#form-new-user').submit(function (e) {
	//gather inputs
	var username = 		$('#form-new-user .username').val();
	var password1 = 	$('#form-new-user .password1').val();
	var password2 = 	$('#form-new-user .password2').val();
	var addCards = 		$('#form-new-user .can-add-cards input:checked').val();
	var removeCards = 	$('#form-new-user .can-remove-cards input:checked').val();
	var chargeCards = 	$('#form-new-user .can-charge-cards input:checked').val();
	var reports = 		$('#form-new-user .can-view-reports input:checked').val();
	var admin = 		$('#form-new-user .is-admin input:checked').val();
	var active = 		$('#form-new-user .is-active input:checked').val();
	var msgElem = 		$('#form-new-user .msg');
	var submit = 		$('#form-new-user-submit');

	//check if username is an email
	if (validateEmail(username) === false) {
		e.preventDefault();
		showModalMessage("You must provide an email address as a username.", "danger", msgElem);
		return false;
	}

	//validate password
	//check if passwords match
	if (doWordsMatch(password1, password2) === false) {
		e.preventDefault();
		showModalMessage("The passwords do not match.", "danger", msgElem);
		return false;
	}

	//make sure the password is long enough
	if (isLongPassword(password1) === false) {
		e.preventDefault();
		showModalMessage("Your password is too short. It must be at least " + MIN_PASSWORD_LENGTH + " characters.", "danger", msgElem);
		return false;
	}

	//make sure the passwords are not very easy to guess
	if (isSimplePassword(password1) === true) {
		e.preventDefault();
		showModalMessage("Your password too simple. Choose a more complex password.", "danger", msgElem);
		return false;
	}

	//clear any alerts
	msgElem.html('');

	//add user via ajax call
	e.preventDefault();
	$.ajax({
		type: 	"POST",
		url: 	"/users/add/",
		data: {
			username: 		username,
			password1: 		password1,
			password2: 		password2,
			addCards: 		addCards,
			removeCards: 	removeCards,
			chargeCards: 	chargeCards,
			reports: 		reports,
			admin: 			admin,
			active: 		active
		},
		beforeSend: function() {
			//disable add user btn and show working message
			submit.attr("disabled", true);
			showModalMessage("Saving user...", "info", msgElem);
			return;
		},
		error: function (r) {
			var j = JSON.parse(r['responseText']);
			if (j['ok'] === false) {
				showModalMessage(j['data']['error_msg'], 'danger', msgElem);
				console.log(j);
				return;
			}

			submit.attr("disabled", true);
			return;
		},
		success: function (r) {
			showModalMessage("New user was saved sucessfully!", "success", msgElem);

			//reset inputs
			//hide success message
			setTimeout(function() {
				submit.attr("disabled", false);
				resetAddUserModal();
			}, 3000);
		}
	});
	return false;
});

//RESET NEW USER MODAL TO DEFAULT VALUES
function resetAddUserModal() {
	$('#form-new-user .username').val('');
	$('#form-new-user .password1').val('');
	$('#form-new-user .password2').val('');
	$('#form-new-user .default').attr("checked", true).parent('label').addClass('active').siblings('label').removeClass('active')
	$('.msg').html('');
	return;
}

//RESET ADD NEW USER MODAL IF THE MODAL CLOSES
$('#modal-new-user').on('hidden.bs.modal', function() {
	resetAddUserModal();
	return;
});

//*******************************************************************************
//CHANGE USER PASSWORD

//GET LIST OF USERS WHEN MODALS OPEN
$('#modal-change-pwd, #modal-update-user').on('show.bs.modal', function() {
	getUsers()
	return;
});

//GET LIST OF USERNAMES AND IDS
//fill in the drop downs for editing users and changing passwords
function getUsers() {
	//there are two of these selects (change pwd & update user)
	var userList = $('.user-list');

	$.ajax({
		type: 	"GET",
		url: 	"/users/get/all/",
		dataType: "json",
		beforeSend: function() {
			userList.html('<option value="0">Loading...</option>').attr('disabled', true);
			return;
		},
		error: function (r) {
			userList.html('<option value="0">Error (please see dev tools)</option>');
			console.log(r);
			return;
		},
		success: function (r) {
			//clear options
			userList.html('');
			userList.append("<option value='0'>Please choose...</option>").attr('disabled', false);

			//display list of users in selects
			var users = r['data'];
			users.forEach(function (u, index) {
				userList.append('<option value="' + u['id'] + '">' + u['username'] + '</option>');
				return;
			});

			return;
		}
	});
}

//CHANGE A USERS PASSWORD
//validate before submitting ajax
$('#form-change-pwd').submit(function (e) {
	//get values
	var id = 		$('#form-change-pwd .user-list').val();
	var pass1 = 	$('#form-change-pwd .password1').val();
	var pass2 = 	$('#form-change-pwd .password2').val();
	var msgElem = 	$('#form-change-pwd .msg');
	var submit = 	$('#change-password-submit');

	//validate password
	//check if passwords match
	if (doWordsMatch(pass1, pass2) === false) {
		e.preventDefault();
		showModalMessage("The passwords do not match.", "danger", msgElem);
		return false;
	}

	//make sure the password is long enough
	if (isLongPassword(pass1) === false) {
		e.preventDefault();
		showModalMessage("Your password is too short. It must be at least " + MIN_PASSWORD_LENGTH + " characters.", "danger", msgElem);
		return false;
	}

	//make sure the passwords are not very easy to guess
	if (isSimplePassword(pass1) === true) {
		e.preventDefault();
		showModalMessage("Your password too simple. Choose a more complex password.", "danger", msgElem);
		return false;
	}

	//ajax to update db
	$.ajax({
		type: 	"POST",
		url: 	"/users/change-pwd/",
		data: {
			userId: id,
			pass1: 	pass1,
			pass2: 	pass2
		},
		beforeSend: function () {
			submit.attr("disabled", true);
			return;
		},
		error: function (r) {
			showModalMessage("An error occured while trying to update this user's password.", "danger", msgElem);
			console.log(r);
			return;
		},
		success: function (r) {
			showModalMessage("This user's password has been updated.", "success", msgElem);

			//reset inputs
			//hide success message
			setTimeout(function() {
				submit.attr("disabled", false);
				resetChangePwdModal();
			}, 3000);
		}
	});

	e.preventDefault();
	return false;
});

//RESET CHANGE PASSWORD MODAL TO DEFAULT VALUES
function resetChangePwdModal() {
	$('.user-list').val('0');
	$('#form-change-pwd .password1').val('');
	$('#form-change-pwd .password2').val('');
	$('.msg').html('');
	return;
}

//RESET CHANGE USER MODAL IF THE MODAL CLOSES
$('#modal-change-pwd').on('hidden.bs.modal', function() {
	resetAddUserModal();
	return;
});

//*******************************************************************************
//CHANGE USER PERMISSIONS

//RESET UPDATE USER MODAL TO DEFAULT VALUES
function resetUpdateUserModal() {
	$('#form-update-user label.btn').attr('disabled', true).removeClass('active');
	$('#form-update-user input[type=radio]').attr('disabled', true).attr('checked', false);
	$('.msg').html('');
	$('#update-user-submit').attr('disabled', true);
	return;
}

//RESET UPDATE USER MODAL IF THE MODAL CLOSES
$('#modal-update-user').on('hidden.bs.modal', function() {
	resetUpdateUserModal();
	return;
});

//GET USER DATA ON SELECTION
//set access control toggles to user's current permissions
//if default user is chosen, 
$('#form-update-user').on('change', '.user-list', function() {
	//get user id from select value
	var userId = 	$(this).val();
	var msgElem = 	$('#form-update-user .msg');

	//if user choosed default option, disable everything
	if (userId === 0) {
		resetUpdateUserModal();
		return;
	}

	//get user data from ajax
	$.ajax({
		type: 	"GET",
		url: 	"/users/get/",
		data: 	{
			userId: userId
		},
		beforeSend: function() {
			resetUpdateUserModal();
			showModalMessage("Retrieving user's permissions...", "info", msgElem);
			return;
		},
		error: function(r) {
			showModalMessage("An error occured while trying to retrieve this users data. Please try again.", "danger", msgElem);
			console.log(r);
			return;
		},
		dataType: "json",
		success: function (j) {
			//hide the alert msg
			msgElem.html('');

			//enable access control toggles
			//enable save btn
			//select the right option again since the entire form was reset in beforeSend
			$('#form-update-user label.btn').attr('disabled', false);
			$('#form-update-user input[type=radio]').attr('disabled', false);
			$('#update-user-submit').attr('disabled', false);

			//make the toggles reflect the user's permissions
			var data = j['data']
			if (data['add_cards']) {
				$('#form-update-user .can-add-cards input[value=true]').attr('checked', true).parent().addClass('active');
			}
			else {
				$('#form-update-user .can-add-cards input[value=false]').attr('checked', true).parent().addClass('active');
			}

			if (data['remove_cards']) {
				$('#form-update-user .can-remove-cards input[value=true]').attr('checked', true).parent().addClass('active');
			}
			else {
				$('#form-update-user .can-remove-cards input[value=false]').attr('checked', true).parent().addClass('active');
			}

			if (data['charge_cards']) {
				$('#form-update-user .can-charge-cards input[value=true]').attr('checked', true).parent().addClass('active');
			}
			else {
				$('#form-update-user .can-charge-cards input[value=false]').attr('checked', true).parent().addClass('active');
			}

			if (data['view_reports']) {
				$('#form-update-user .can-view-reports input[value=true]').attr('checked', true).parent().addClass('active');
			}
			else {
				$('#form-update-user .can-view-reports input[value=false]').attr('checked', true).parent().addClass('active');
			}

			if (data['is_admin']) {
				$('#form-update-user .is-admin input[value=true]').attr('checked', true).parent().addClass('active');
			}
			else {
				$('#form-update-user .is-admin input[value=false]').attr('checked', true).parent().addClass('active');
			}

			if (data['is_active']) {
				$('#form-update-user .is-active input[value=true]').attr('checked', true).parent().addClass('active');
			}
			else {
				$('#form-update-user .is-active input[value=false]').attr('checked', true).parent().addClass('active');
			}







			console.log(j);



		}
	});



	return;
});
