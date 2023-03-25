
 
 name: Issue Report
description: "Create a report to help us improve."
body:
  - type:  checkboxes
    id: terms
    attributes:
      label: Welcome
      options:
        - label: Yes, I'm using the latest major release. Only such installations are supported.
          required: true
        - label: Yes, I'm using the supported system. Only such systems are supported.
          required: true
        - label: Yes, I have read all WIKI document,nothing can help me in my problem.
          required: true
        - label: Yes, I've searched similar issues on GitHub and didn't find any.
          required: true
        - label: Yes, I've included all information below (version, config, log, etc).
          required: true

  - type: textarea
    id: problem
    attributes:
      label: Description of the problem,screencshot would be good 
      placeholder: Your problem description
    validations:
      required: true

  - type: textarea
    id: version
    attributes:
      label: Version of 3x-ui
      value: |-
        <details>

	- OS: [e.g. ubuntu 22]
	- 3x-ui [e.g. v1.1.2]

        </details>
    validations:
      required: true

  - type: textarea
    id: log
    attributes:
      label: x-ui Log reports or xray log
      value: |-
        <details>

        ```console
        # x-ui log
		# copy and paste log reports from your panel (index page)
        ```

        </details>
    validations:
      required: true
