package mailer

import (
	"bytes"
	"cinemabooking/internal/domain"
	"context"
	"fmt"
	"html/template"
	"time"

	"github.com/resend/resend-go/v3"
)

type Service struct {
	client    *resend.Client
	fromEmail string
	fromName  string
}

func New(apiKey, fromEmail, fromName string) *Service {
	return &Service{
		client:    resend.NewClient(apiKey),
		fromEmail: fromEmail,
		fromName:  fromName,
	}
}

// template helpers used across all email templates
var tmplFuncs = template.FuncMap{
	"formatTime": func(t time.Time) string {
		return t.Format("Mon, 2 Jan 2006 at 3:04 PM")
	},
}

func renderTemplate(tmpl *template.Template, data any) (string, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (s *Service) from() string {
	return fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail)
}

func (s *Service) send(ctx context.Context, to, subject, html string) error {
	_, err := s.client.Emails.Send(&resend.SendEmailRequest{
		From:    s.from(),
		To:      []string{to},
		Subject: subject,
		Html:    html,
	})
	return err
}

func (s *Service) SendBookingConfirmed(ctx context.Context, p domain.BookingConfirmedPayload) error {
	html, err := renderTemplate(bookingConfirmedTmpl, p)
	if err != nil {
		return fmt.Errorf("rendering booking confirmed template: %w", err)
	}

	return s.send(
		ctx,
		// p.User.Email,
		"rp0096745@gmail.com", // temporary
		fmt.Sprintf("Your tickets for %s are confirmed!", p.Movie.Title),
		html,
	)
}

var bookingConfirmedTmpl = template.Must(template.New("booking_confirmed").Funcs(tmplFuncs).Parse(`
<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1.0"/>
</head>
<body style="margin:0;padding:0;background:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;">
  <table width="100%" cellpadding="0" cellspacing="0">
    <tr><td align="center" style="padding:32px 16px;">
      <table width="560" cellpadding="0" cellspacing="0" style="background:#fff;border-radius:12px;overflow:hidden;">

        <!-- header -->
        <tr><td style="background:#1a1a2e;padding:28px 32px;">
          <p style="margin:0;color:#fff;font-size:22px;font-weight:600;">🎬 CinemaBook</p>
          <p style="margin:6px 0 0;color:#a5a5b5;font-size:14px;">Booking Confirmation</p>
        </td></tr>

        <!-- greeting -->
        <tr><td style="padding:28px 32px 0;">
          <p style="margin:0;font-size:16px;color:#111;">Hi {{.User.Name}},</p>
          <p style="margin:10px 0 0;font-size:15px;color:#444;line-height:1.6;">
            Your booking is confirmed! Here are your ticket details.
          </p>
        </td></tr>

        <!-- movie info -->
        <tr><td style="padding:20px 32px 0;">
          <table width="100%" cellpadding="0" cellspacing="0" style="background:#f8f8fa;border-radius:8px;overflow:hidden;">
            <tr>
              <td style="padding:20px;">
                <p style="margin:0;font-size:18px;font-weight:600;color:#111;">{{.Movie.Title}}</p>
                <p style="margin:4px 0 0;font-size:13px;color:#666;">{{.Movie.Language}} &nbsp;·&nbsp; {{.Movie.DurationMin}} mins</p>
                <table width="100%" cellpadding="0" cellspacing="0" style="margin-top:16px;">
                  <tr>
                    <td style="width:50%;vertical-align:top;">
                      <p style="margin:0;font-size:11px;color:#888;text-transform:uppercase;letter-spacing:.05em;">Date &amp; Time</p>
                      <p style="margin:4px 0 0;font-size:14px;color:#111;font-weight:500;">{{formatTime .Showtime.StartsAt}}</p>
                    </td>
                    <td style="width:50%;vertical-align:top;">
                      <p style="margin:0;font-size:11px;color:#888;text-transform:uppercase;letter-spacing:.05em;">Theatre</p>
                      <p style="margin:4px 0 0;font-size:14px;color:#111;font-weight:500;">{{.Theatre.Name}}</p>
                      <p style="margin:2px 0 0;font-size:12px;color:#666;">{{.Hall.Name}} &nbsp;·&nbsp; {{.Theatre.City}}</p>
                    </td>
                  </tr>
                </table>
              </td>
            </tr>
          </table>
        </td></tr>

        <!-- seats -->
        <tr><td style="padding:20px 32px 0;">
          <p style="margin:0 0 10px;font-size:13px;color:#888;text-transform:uppercase;letter-spacing:.05em;">Your Seats</p>
          <table width="100%" cellpadding="0" cellspacing="0">
            {{range .Seats}}
            <tr>
              <td style="padding:8px 12px;border-bottom:1px solid #f0f0f0;">
                <span style="font-size:14px;font-weight:500;color:#111;">
                  {{.RowLabel}}{{.ColNumber}}
                </span>
                <span style="margin-left:8px;font-size:12px;color:#888;text-transform:capitalize;">
                  {{.SeatType}}
                </span>
              </td>
              <td style="padding:8px 12px;border-bottom:1px solid #f0f0f0;text-align:right;">
                <span style="font-size:14px;color:#111;">₹{{.Price}}</span>
              </td>
            </tr>
            {{end}}
            <tr>
              <td style="padding:12px;font-weight:600;font-size:15px;color:#111;">Total</td>
              <td style="padding:12px;text-align:right;font-weight:600;font-size:15px;color:#111;">₹{{.TotalAmount}}</td>
            </tr>
          </table>
        </td></tr>

        <!-- qr code -->
        <tr><td style="padding:24px 32px;text-align:center;">
          <p style="margin:0 0 14px;font-size:13px;color:#888;">Show this QR code at the theatre entrance</p>
          <img src="{{.QRCodeURL}}" width="180" height="180" alt="Ticket QR Code"
               style="border:1px solid #eee;border-radius:8px;"/>
          <p style="margin:12px 0 0;font-size:12px;color:#aaa;">Booking ID: {{.Booking.ID}}</p>
        </td></tr>

        <!-- footer -->
        <tr><td style="padding:20px 32px;background:#f8f8fa;border-top:1px solid #eee;">
          <p style="margin:0;font-size:12px;color:#999;text-align:center;line-height:1.6;">
            Need help? Reply to this email or contact support.<br/>
            Cancellations are allowed up to 2 hours before the show.
          </p>
        </td></tr>

      </table>
    </td></tr>
  </table>
</body>
</html>
`))

func (s *Service) SendBookingCancelled(ctx context.Context, p domain.BookingCancelledPayload) error {
	html, err := renderTemplate(bookingCancelledTmpl, p)
	if err != nil {
		return fmt.Errorf("rendering booking cancelled template: %w", err)
	}

	return s.send(
		ctx,
		// p.User.Email,
		"rp0096745@gmail.com", // temporary
		fmt.Sprintf("Your booking for %s has been cancelled", p.Movie.Title),
		html,
	)
}

var bookingCancelledTmpl = template.Must(template.New("booking_cancelled").Funcs(tmplFuncs).Parse(`
<!DOCTYPE html>
<html>
<body style="margin:0;padding:0;background:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;">
  <table width="100%" cellpadding="0" cellspacing="0">
    <tr><td align="center" style="padding:32px 16px;">
      <table width="560" cellpadding="0" cellspacing="0" style="background:#fff;border-radius:12px;overflow:hidden;">

        <tr><td style="background:#1a1a2e;padding:28px 32px;">
          <p style="margin:0;color:#fff;font-size:22px;font-weight:600;">🎬 CinemaBook</p>
          <p style="margin:6px 0 0;color:#a5a5b5;font-size:14px;">Booking Cancelled</p>
        </td></tr>

        <tr><td style="padding:28px 32px;">
          <p style="margin:0;font-size:16px;color:#111;">Hi {{.User.Name}},</p>
          <p style="margin:10px 0 0;font-size:15px;color:#444;line-height:1.6;">
            Your booking for <strong>{{.Movie.Title}}</strong> has been cancelled.
          </p>
          {{if .RefundAmount}}
          <table width="100%" cellpadding="0" cellspacing="0"
                 style="margin-top:20px;background:#f0fdf4;border-radius:8px;border:1px solid #bbf7d0;">
            <tr><td style="padding:16px 20px;">
              <p style="margin:0;font-size:14px;color:#166534;font-weight:500;">
                Refund of ₹{{.RefundAmount}} will be credited within {{.RefundDays}} business days.
              </p>
            </td></tr>
          </table>
          {{end}}
          <p style="margin:20px 0 0;font-size:13px;color:#666;">Booking ID: {{.Booking.ID}}</p>
        </td></tr>

        <tr><td style="padding:20px 32px;background:#f8f8fa;border-top:1px solid #eee;">
          <p style="margin:0;font-size:12px;color:#999;text-align:center;">
            Questions? Reply to this email and we'll help you out.
          </p>
        </td></tr>

      </table>
    </td></tr>
  </table>
</body>
</html>
`))

func (s *Service) SendShowtimeReminder(ctx context.Context, p domain.ShowtimeReminderPayload) error {
	html, err := renderTemplate(showtimeReminderTmpl, p)
	if err != nil {
		return fmt.Errorf("rendering reminder template: %w", err)
	}

	return s.send(
		ctx,
		p.User.Email,
		fmt.Sprintf("🎬 Your show starts in 2 hours — %s", p.Movie.Title),
		html,
	)
}

var showtimeReminderTmpl = template.Must(template.New("showtime_reminder").Funcs(tmplFuncs).Parse(`
<!DOCTYPE html>
<html>
<body style="margin:0;padding:0;background:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;">
  <table width="100%" cellpadding="0" cellspacing="0">
    <tr><td align="center" style="padding:32px 16px;">
      <table width="560" cellpadding="0" cellspacing="0" style="background:#fff;border-radius:12px;overflow:hidden;">

        <tr><td style="background:#1a1a2e;padding:28px 32px;">
          <p style="margin:0;color:#fff;font-size:22px;font-weight:600;">🎬 CinemaBook</p>
          <p style="margin:6px 0 0;color:#a5a5b5;font-size:14px;">Your show is coming up!</p>
        </td></tr>

        <tr><td style="padding:28px 32px;">
          <p style="margin:0;font-size:16px;color:#111;">Hi {{.User.Name}},</p>
          <p style="margin:10px 0 0;font-size:15px;color:#444;line-height:1.6;">
            <strong>{{.Movie.Title}}</strong> starts in 2 hours. Time to head out!
          </p>
          <table width="100%" cellpadding="0" cellspacing="0"
                 style="margin-top:20px;background:#f8f8fa;border-radius:8px;">
            <tr><td style="padding:20px;">
              <table width="100%" cellpadding="0" cellspacing="0">
                <tr>
                  <td style="width:50%;vertical-align:top;">
                    <p style="margin:0;font-size:11px;color:#888;text-transform:uppercase;">Starts at</p>
                    <p style="margin:4px 0 0;font-size:15px;font-weight:600;color:#111;">{{formatTime .Showtime.StartsAt}}</p>
                  </td>
                  <td style="width:50%;vertical-align:top;">
                    <p style="margin:0;font-size:11px;color:#888;text-transform:uppercase;">Theatre</p>
                    <p style="margin:4px 0 0;font-size:15px;font-weight:600;color:#111;">{{.Theatre.Name}}</p>
                    <p style="margin:2px 0 0;font-size:12px;color:#666;">{{.Theatre.Address}}</p>
                  </td>
                </tr>
                <tr><td colspan="2" style="padding-top:16px;">
                  <p style="margin:0;font-size:11px;color:#888;text-transform:uppercase;">Your Seats</p>
                  <p style="margin:4px 0 0;font-size:15px;font-weight:600;color:#111;">
                    {{range $i, $s := .Seats}}{{if $i}}, {{end}}{{$s.RowLabel}}{{$s.ColNumber}}{{end}}
                  </p>
                </td></tr>
              </table>
            </td></tr>
          </table>
          <p style="margin:20px 0 0;font-size:13px;color:#666;">
            Don't forget your QR code — check your confirmation email or the app.
          </p>
        </td></tr>

        <tr><td style="padding:20px 32px;background:#f8f8fa;border-top:1px solid #eee;">
          <p style="margin:0;font-size:12px;color:#999;text-align:center;">Enjoy the show!</p>
        </td></tr>

      </table>
    </td></tr>
  </table>
</body>
</html>
`))

func (s *Service) SendVerificationCode(ctx context.Context, to, code string) error {
	html, err := renderTemplate(verificationTmpl, code)
	if err != nil {
		return fmt.Errorf("rendering verification template: %w", err)
	}

	return s.send(
		ctx,
		// to,
		"rp0096745@gmail.com", // temporary
		"Verify Your Email",
		html,
	)
}

var verificationTmpl = template.Must(template.New("verification").Funcs(tmplFuncs).Parse(`
<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1.0"/>
</head>
<body style="margin:0;padding:0;background:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;">
  <table width="100%" cellpadding="0" cellspacing="0">
    <tr><td align="center" style="padding:32px 16px;">
      <table width="560" cellpadding="0" cellspacing="0" style="background:#fff;border-radius:12px;overflow:hidden;">

        <tr><td style="background:#1a1a2e;padding:28px 32px;">
          <p style="margin:0;color:#fff;font-size:22px;font-weight:600;">🎬 CinemaBook</p>
          <p style="margin:6px 0 0;color:#a5a5b5;font-size:14px;">Email Verification</p>
        </td></tr>

        <tr><td style="padding:28px 32px;">
          <p style="margin:0;font-size:16px;color:#111;">Hi there,</p>
          <p style="margin:10px 0 0;font-size:15px;color:#444;line-height:1.6;">
            Please use the following code to verify your email address.
          </p>

          <table width="100%" cellpadding="0" cellspacing="0" style="margin-top:20px;">
            <tr>
              <td style="padding:12px 16px;background:#f0f0f0;border-radius:6px;text-align:center;">
                <span style="font-size:24px;font-weight:700;letter-spacing:2px;color:#111;">{{.}}</span>
              </td>
            </tr>
          </table>

          <p style="margin:20px 0 0;font-size:13px;color:#666;">
            This code will expire in 10 minutes.
          </p>
          <p style="margin:20px 0 0;font-size:13px;color:#666;">
            If you did not initiate this request, please ignore this email.
          </p>
        </td></tr>

        <tr><td style="padding:20px 32px;background:#f8f8fa;border-top:1px solid #eee;">
          <p style="margin:0;font-size:12px;color:#999;text-align:center;">Thank you,<br/>CinemaBook Team</p>
        </td></tr>

      </table>
    </td></tr>
  </table>
</body>
</html>
`))
