export function InternInsiderComparison() {
  return (
    <div className="table-wrap">
      <table className="table">
        <thead>
          <tr>
            <th>Feature</th>
            <th>OpenIntern</th>
            <th>Intern Insider</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td>Price</td>
            <td>Free</td>
            <td>Paid subscription</td>
          </tr>
          <tr>
            <td>Account to browse</td>
            <td>No</td>
            <td>Required / gated</td>
          </tr>
          <tr>
            <td>Open source</td>
            <td>Apache-2.0</td>
            <td>Closed</td>
          </tr>
          <tr>
            <td>Public API</td>
            <td>Yes</td>
            <td>No</td>
          </tr>
          <tr>
            <td>Bulk export</td>
            <td>Daily JSON/CSV</td>
            <td>Locked</td>
          </tr>
          <tr>
            <td>Apply destination</td>
            <td>Employer site</td>
            <td>In-platform tools</td>
          </tr>
          <tr>
            <td>Recruiter email finder</td>
            <td>No</td>
            <td>Sold as a feature</td>
          </tr>
          <tr>
            <td>AI resume builder</td>
            <td>No</td>
            <td>Sold as a feature</td>
          </tr>
        </tbody>
      </table>
    </div>
  );
}
